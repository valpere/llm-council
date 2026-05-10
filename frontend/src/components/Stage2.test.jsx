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
    // Use one of the still-unimplemented reserved kinds (debate_round) — vote_tally
    // is shipped now, so the unknown-kind view test must use a kind that's still
    // reserved.
    render(<Stage2 kind="debate_round" isLoading={false} />);
    expect(screen.getByText(/view not implemented yet/i)).toBeInTheDocument();
    expect(screen.getByText('debate_round')).toBeInTheDocument();
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

  describe('VoteTallyView (kind="vote_tally")', () => {
    it('renders all clusters with the winner highlighted', () => {
      render(
        <Stage2
          kind="vote_tally"
          voteTally={{
            winner_label: 'Response A',
            clusters: [
              { members: ['Response A', 'Response B'], representative: 'yes', votes: 2 },
              { members: ['Response C'], representative: 'no', votes: 1 },
              { members: ['Response D'], representative: 'maybe', votes: 1 },
            ],
          }}
          isLoading={false}
        />,
      );
      expect(screen.getByText(/Stage 2: Vote Tally/i)).toBeInTheDocument();
      // Winner cluster shows the ✓ marker.
      expect(screen.getByLabelText(/winner/i)).toBeInTheDocument();
      // Vote counts visible.
      expect(screen.getByText(/2 votes/i)).toBeInTheDocument();
      // 'maybe' and 'no' both at 1 vote — getAllByText handles plural.
      expect(screen.getAllByText(/1 vote(?!s)/i).length).toBeGreaterThanOrEqual(2);
      // All three representatives present.
      expect(screen.getByText('yes')).toBeInTheDocument();
      expect(screen.getByText('no')).toBeInTheDocument();
      expect(screen.getByText('maybe')).toBeInTheDocument();
    });

    it('renders a unanimous (single-cluster) tally', () => {
      render(
        <Stage2
          kind="vote_tally"
          voteTally={{
            winner_label: 'Response A',
            clusters: [
              { members: ['Response A', 'Response B', 'Response C'], representative: '42', votes: 3 },
            ],
          }}
          isLoading={false}
        />,
      );
      expect(screen.getByText(/Stage 2: Vote Tally/i)).toBeInTheDocument();
      expect(screen.getByText(/3 votes/i)).toBeInTheDocument();
      expect(screen.getByText('42')).toBeInTheDocument();
      // Exactly one winner marker on a unanimous tally.
      expect(screen.getAllByLabelText(/winner/i)).toHaveLength(1);
    });

    it('truncates long representative content with a Show full answer button', () => {
      const longText =
        'This is a long representative answer that exceeds the truncation threshold so the dispatcher exposes a Show full answer button — '.repeat(2);
      render(
        <Stage2
          kind="vote_tally"
          voteTally={{
            winner_label: 'Response A',
            clusters: [{ members: ['Response A'], representative: longText, votes: 3 }],
          }}
          isLoading={false}
        />,
      );
      expect(screen.getByRole('button', { name: /Show full answer/i })).toBeInTheDocument();
    });
  });
});
