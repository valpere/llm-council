// Tests for the App.jsx state machine.
//
// App.jsx owns all SSE event routing and the conversation-closure flag. These
// tests mock the `./api` module entirely, then drive a synthetic event stream
// through the `onEvent` callback and assert the resulting UI/prop state.

import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

// Hoisted mock factory — vi.mock is hoisted above imports, so the spies it
// references must be hoisted too.
const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    listConversations: vi.fn(),
    createConversation: vi.fn(),
    getConversation: vi.fn(),
    sendMessage: vi.fn(),
    sendMessageStream: vi.fn(),
  },
}));

vi.mock('./api', () => ({ api: mockApi }));

// ── helpers ────────────────────────────────────────────────────────────────

function makeConversation(overrides = {}) {
  return {
    id: 'conv-1',
    title: 'Test Conversation',
    created_at: '2026-05-02T12:00:00Z',
    closed: false,
    messages: [],
    ...overrides,
  };
}

// scriptedStream returns a sendMessageStream stub that synchronously fires the
// given events through the onEvent callback, then resolves.
function scriptedStream(events) {
  return async (_id, _body, onEvent) => {
    for (const [type, payload] of events) {
      onEvent(type, payload);
    }
  };
}

beforeEach(() => {
  Object.values(mockApi).forEach((fn) => fn.mockReset());
  // localStorage is jsdom-backed; clear so theme reads fresh.
  localStorage.clear();
});

// ── rendering bootstrapping ────────────────────────────────────────────────

describe('App initial mount', () => {
  it('lists conversations on mount and renders the empty selector', async () => {
    mockApi.listConversations.mockResolvedValue([
      { id: 'conv-1', title: 'One', created_at: '2026-05-02T12:00:00Z' },
    ]);

    render(<App />);

    await waitFor(() => expect(mockApi.listConversations).toHaveBeenCalled());
    // Empty-state copy shown until a conversation is selected.
    expect(
      await screen.findByText(/Select or create a conversation/i),
    ).toBeInTheDocument();
  });
});

// ── stage3_complete sets closed: true ──────────────────────────────────────

describe('SSE handler: stage3_complete', () => {
  it('marks the conversation as closed and disables ChatInterface input', async () => {
    mockApi.listConversations.mockResolvedValue([
      { id: 'conv-1', title: 'One', created_at: '2026-05-02T12:00:00Z' },
    ]);
    mockApi.getConversation.mockResolvedValue(makeConversation());
    mockApi.createConversation.mockResolvedValue({
      id: 'conv-2',
      title: 'New',
      created_at: '2026-05-02T12:00:00Z',
    });
    mockApi.sendMessageStream.mockImplementation(
      scriptedStream([
        ['stage1_complete', { type: 'stage1_complete', data: [{ label: 'A' }] }],
        [
          'stage2_complete',
          {
            type: 'stage2_complete',
            data: [],
            metadata: { label_to_model: {}, aggregate_rankings: [], consensus_w: 1 },
          },
        ],
        [
          'stage3_complete',
          {
            type: 'stage3_complete',
            data: { content: 'final answer', model: 'openai/gpt-4o-mini' },
          },
        ],
      ]),
    );

    const user = userEvent.setup();
    render(<App />);

    // Pick the existing conversation so a ChatInterface mounts.
    const sidebarItem = await screen.findByRole('button', { name: /One/ });
    await user.click(sidebarItem);

    // Wait for the input to be enabled before typing.
    const input = await screen.findByPlaceholderText(/Ask a question/i);
    await waitFor(() => expect(input).not.toBeDisabled());

    await user.type(input, 'hello');
    await user.click(screen.getByRole('button', { name: /Send/i }));

    // After the scripted stage3_complete fires, the input should be disabled
    // because App sets currentConversation.closed = true.
    await waitFor(() => {
      const closedInput = screen.getByPlaceholderText(/conversation has ended/i);
      expect(closedInput).toBeDisabled();
    });
  });
});

// ── stage0_round_complete populates pendingClarification ───────────────────

describe('SSE handler: stage0_round_complete', () => {
  it('populates pendingClarification and clears stage0/stage1 loading flags', async () => {
    mockApi.listConversations.mockResolvedValue([
      { id: 'conv-1', title: 'One', created_at: '2026-05-02T12:00:00Z' },
    ]);
    mockApi.getConversation.mockResolvedValue(makeConversation());
    mockApi.sendMessageStream.mockImplementation(
      scriptedStream([
        [
          'stage0_round_complete',
          {
            type: 'stage0_round_complete',
            data: {
              round: 1,
              questions: [{ id: 'q1', text: 'Which framework?' }],
            },
          },
        ],
      ]),
    );

    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByRole('button', { name: /One/ }));

    const input = await screen.findByPlaceholderText(/Ask a question/i);
    await waitFor(() => expect(input).not.toBeDisabled());

    await user.type(input, 'help me');
    await user.click(screen.getByRole('button', { name: /Send/i }));

    // Once stage0_round_complete is processed, ChatInterface swaps the input
    // placeholder to the "Answer the questions above…" copy.
    expect(
      await screen.findByPlaceholderText(/Answer the questions above/i),
    ).toBeInTheDocument();
    // The clarification question text appears in the assistant-message area.
    expect(await screen.findByText(/Which framework\?/)).toBeInTheDocument();
  });
});

// ── error handler clears loading and surfaces the message ──────────────────

describe('SSE handler: error', () => {
  it('renders the error message when the stream emits a typed error event', async () => {
    mockApi.listConversations.mockResolvedValue([
      { id: 'conv-1', title: 'One', created_at: '2026-05-02T12:00:00Z' },
    ]);
    mockApi.getConversation.mockResolvedValue(makeConversation());
    mockApi.sendMessageStream.mockImplementation(
      scriptedStream([['error', { type: 'error', message: 'council quorum not met' }]]),
    );

    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByRole('button', { name: /One/ }));

    const input = await screen.findByPlaceholderText(/Ask a question/i);
    await waitFor(() => expect(input).not.toBeDisabled());

    await user.type(input, 'hi');
    await user.click(screen.getByRole('button', { name: /Send/i }));

    // The error text appears via the Stage3 error renderer.
    expect(await screen.findByText(/council quorum not met/i)).toBeInTheDocument();
    // Input must come back online (loading flags all cleared).
    await waitFor(() =>
      expect(screen.getByPlaceholderText(/Ask a question/i)).not.toBeDisabled(),
    );
  });
});

// ── isConversationClosed prop propagation on load ──────────────────────────

describe('loadConversation propagates closed flag', () => {
  it('disables the input when the loaded conversation has closed: true', async () => {
    mockApi.listConversations.mockResolvedValue([
      { id: 'conv-1', title: 'Closed One', created_at: '2026-05-02T12:00:00Z' },
    ]);
    mockApi.getConversation.mockResolvedValue(
      makeConversation({ closed: true, messages: [] }),
    );

    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByRole('button', { name: /Closed One/ }));

    // The closed-state placeholder should appear without any send action.
    const input = await screen.findByPlaceholderText(/conversation has ended/i);
    expect(input).toBeDisabled();
    // Send button is disabled too.
    const sendBtn = screen.getByRole('button', { name: /Send/i });
    expect(sendBtn).toBeDisabled();
  });

  it('keeps the input enabled when closed is false (default)', async () => {
    mockApi.listConversations.mockResolvedValue([
      { id: 'conv-1', title: 'Open One', created_at: '2026-05-02T12:00:00Z' },
    ]);
    mockApi.getConversation.mockResolvedValue(
      makeConversation({ closed: false }),
    );

    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByRole('button', { name: /Open One/ }));

    const input = await screen.findByPlaceholderText(/Ask a question/i);
    await waitFor(() => expect(input).not.toBeDisabled());
  });
});

// ── act() guard for fail-fast on warnings ──────────────────────────────────

// React Testing Library wraps render/userEvent in act for us; if a state
// update slips outside act the test logs a warning. We do not assert on those
// warnings here, but the imports above ensure act is in scope for any future
// test that needs to wrap an asynchronous setState directly.
void act;
