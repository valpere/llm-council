// Tests for Stage2.jsx — the dispatcher routes by `kind` to one of three
// sub-renderers (PeerRankingView, RoleStubView, UnknownKindView). These tests
// drive each branch without mounting the rest of App.

import { render, screen } from '@testing-library/react';
import Stage2 from './Stage2';

describe('Stage2 dispatcher', () => {
  it('routes kind="peer_ranking" to the peer-ranking view', () => {
    render(
      <Stage2
        kind="peer_ranking"
        rankings={[
          { reviewer_label: 'Response A', rankings: ['Response A', 'Response B'] },
        ]}
        labelToModel={{ 'Response A': 'openai/gpt-4o', 'Response B': 'anthropic/claude-haiku-4-5' }}
        aggregateRankings={[
          { model: 'openai/gpt-4o', score: 1.5 },
          { model: 'anthropic/claude-haiku-4-5', score: 2.5 },
        ]}
        consensusW={0.8}
        isLoading={false}
      />,
    );
    // Peer-ranking view renders the strong-consensus pill.
    expect(screen.getByText(/Stage 2: Peer Rankings/i)).toBeInTheDocument();
    expect(screen.getByText(/W=0.80/i)).toBeInTheDocument();
  });

  it('routes kind="role_stub" to the role-stub view', () => {
    render(<Stage2 kind="role_stub" isLoading={false} />);
    expect(
      screen.getByText(/roles are complementary — no peer ranking/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Stage 2: Peer Rankings/i)).not.toBeInTheDocument();
  });

  it('routes an unknown kind to the unknown-kind view, surfacing the kind name', () => {
    render(<Stage2 kind="vote_tally" isLoading={false} />);
    expect(screen.getByText(/view not implemented yet/i)).toBeInTheDocument();
    // The kind name itself must be visible (inside a <code> element).
    expect(screen.getByText('vote_tally')).toBeInTheDocument();
  });

  it('defaults to peer_ranking when kind is undefined (old-backend safety net)', () => {
    render(
      <Stage2
        rankings={[
          { reviewer_label: 'Response A', rankings: ['Response A'] },
        ]}
        labelToModel={{ 'Response A': 'openai/gpt-4o' }}
        aggregateRankings={[]}
        consensusW={0.0}
        isLoading={false}
      />,
    );
    // Falls through to PeerRankingView even though kind is undefined.
    expect(screen.getByText(/Stage 2: Peer Rankings/i)).toBeInTheDocument();
  });
});
