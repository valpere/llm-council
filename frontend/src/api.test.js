// Tests for the SSE adapter boundary at frontend/src/api.js.
//
// The architecture rule says api.js is the only place that calls fetch and
// parses raw SSE — everything above receives the typed `onEvent(type, event)`
// callback. These tests pin that contract.

import { ReadableStream } from 'node:stream/web';
import { api } from './api';

// ── helpers ────────────────────────────────────────────────────────────────

function jsonResponse(body, init = { status: 200 }) {
  return new Response(JSON.stringify(body), {
    status: init.status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function notOkResponse(status = 500) {
  return new Response('error', { status });
}

// streamResponse builds a Response with a body that emits the given chunks
// one at a time. Chunks are Uint8Arrays so the test can simulate multi-byte
// boundary cases (a `data:` line split across two chunks, etc.).
function streamResponse(chunks) {
  const stream = new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk);
      controller.close();
    },
  });
  return new Response(stream, {
    status: 200,
    headers: { 'Content-Type': 'text/event-stream' },
  });
}

const enc = (s) => new TextEncoder().encode(s);

beforeEach(() => {
  vi.restoreAllMocks();
});

// ── listConversations ──────────────────────────────────────────────────────

describe('api.listConversations', () => {
  it('GETs /api/conversations and returns parsed JSON', async () => {
    const fetchSpy = vi.fn().mockResolvedValueOnce(jsonResponse([{ id: 'a' }]));
    vi.stubGlobal('fetch', fetchSpy);

    const result = await api.listConversations();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy).toHaveBeenCalledWith('/api/conversations');
    expect(result).toEqual([{ id: 'a' }]);
  });

  it('throws on non-OK response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(notOkResponse(503)));
    await expect(api.listConversations()).rejects.toThrow(/Failed to list/);
  });
});

// ── createConversation ─────────────────────────────────────────────────────

describe('api.createConversation', () => {
  it('POSTs /api/conversations with empty JSON body and returns parsed result', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ id: 'new-id', created_at: 'now' }));
    vi.stubGlobal('fetch', fetchSpy);

    const result = await api.createConversation();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('/api/conversations');
    expect(init.method).toBe('POST');
    expect(init.headers['Content-Type']).toBe('application/json');
    expect(init.body).toBe('{}');
    expect(result).toEqual({ id: 'new-id', created_at: 'now' });
  });

  it('throws on non-OK response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(notOkResponse(500)));
    await expect(api.createConversation()).rejects.toThrow(/Failed to create/);
  });
});

// ── getConversation ────────────────────────────────────────────────────────

describe('api.getConversation', () => {
  it('GETs /api/conversations/{id}', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ id: 'abc', messages: [] }));
    vi.stubGlobal('fetch', fetchSpy);

    const result = await api.getConversation('abc');

    expect(fetchSpy).toHaveBeenCalledWith('/api/conversations/abc');
    expect(result).toEqual({ id: 'abc', messages: [] });
  });

  it('throws on non-OK response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(notOkResponse(404)));
    await expect(api.getConversation('missing')).rejects.toThrow(/Failed to get/);
  });
});

// ── sendMessage ────────────────────────────────────────────────────────────

describe('api.sendMessage', () => {
  it('POSTs /api/conversations/{id}/message with the content body', async () => {
    const fetchSpy = vi.fn().mockResolvedValueOnce(jsonResponse({ ok: true }));
    vi.stubGlobal('fetch', fetchSpy);

    await api.sendMessage('abc', 'hello');

    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('/api/conversations/abc/message');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ content: 'hello' });
  });

  it('throws on non-OK response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(notOkResponse(400)));
    await expect(api.sendMessage('abc', 'x')).rejects.toThrow(/Failed to send/);
  });
});

// ── sendMessageStream ──────────────────────────────────────────────────────

describe('api.sendMessageStream', () => {
  it('parses a multi-event SSE stream and fires onEvent once per data line', async () => {
    const events = [
      enc('data: {"type":"stage1_complete","data":[1]}\n\n'),
      enc('data: {"type":"stage2_complete","data":[2]}\n\n'),
      enc('data: {"type":"stage3_complete","data":{"x":1}}\n\n'),
      enc('data: {"type":"complete"}\n\n'),
    ];
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(streamResponse(events)));

    const calls = [];
    const onEvent = vi.fn((type, event) => calls.push([type, event]));

    await api.sendMessageStream('abc', { content: 'hi' }, onEvent);

    expect(calls).toHaveLength(4);
    expect(calls[0][0]).toBe('stage1_complete');
    expect(calls[0][1]).toEqual({ type: 'stage1_complete', data: [1] });
    expect(calls[1][0]).toBe('stage2_complete');
    expect(calls[2][0]).toBe('stage3_complete');
    expect(calls[3][0]).toBe('complete');
  });

  it('handles a data: line split across two chunk boundaries', async () => {
    // Single SSE record split mid-payload — exercises the `stream: true` decode
    // path that keeps a partial line in the buffer until the next chunk.
    const full = 'data: {"type":"stage1_complete","data":["A"]}\n\n';
    const split = Math.floor(full.length / 2);
    const chunks = [enc(full.slice(0, split)), enc(full.slice(split))];

    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(streamResponse(chunks)));

    const onEvent = vi.fn();
    await api.sendMessageStream('abc', { content: 'hi' }, onEvent);

    expect(onEvent).toHaveBeenCalledTimes(1);
    expect(onEvent).toHaveBeenCalledWith('stage1_complete', {
      type: 'stage1_complete',
      data: ['A'],
    });
  });

  it('logs and skips malformed JSON without throwing', async () => {
    const events = [
      enc('data: {bogus json\n\n'),
      enc('data: {"type":"complete"}\n\n'),
    ];
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(streamResponse(events)));
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const onEvent = vi.fn();

    await expect(
      api.sendMessageStream('abc', { content: 'hi' }, onEvent),
    ).resolves.toBeUndefined();

    expect(errSpy).toHaveBeenCalled(); // malformed JSON warning logged
    expect(onEvent).toHaveBeenCalledTimes(1);
    expect(onEvent).toHaveBeenCalledWith('complete', { type: 'complete' });
  });

  it('throws on non-OK response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(notOkResponse(500)));
    await expect(
      api.sendMessageStream('abc', { content: 'hi' }, vi.fn()),
    ).rejects.toThrow(/Failed to send/);
  });

  it('forwards the body verbatim (clarification answers shape)', async () => {
    const fetchSpy = vi
      .fn()
      .mockResolvedValueOnce(streamResponse([enc('data: {"type":"complete"}\n\n')]));
    vi.stubGlobal('fetch', fetchSpy);

    await api.sendMessageStream(
      'abc',
      { answers: [{ id: 'q1', text: 'yes' }] },
      vi.fn(),
    );

    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('/api/conversations/abc/message/stream');
    expect(JSON.parse(init.body)).toEqual({
      answers: [{ id: 'q1', text: 'yes' }],
    });
  });
});
