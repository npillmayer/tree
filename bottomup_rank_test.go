package tree

import (
	"sync"
	"testing"
)

// Reproduce a backlog of parent notifications after both children complete.
// All notifications see the parent as ready; only one may run its action.
func TestBottomUpClaimsReadyParentOnce(t *testing.T) {
	root := NewNode(0)
	a, b := NewNode(1), NewNode(2)
	root.AddChild(a).AddChild(b)
	var mu sync.Mutex
	calls := map[*Node[int]]int{}
	data := &bottomUpFilterData[int]{childrenDict: newRankMap[int](), action: func(n, p *Node[int], pos int) (*Node[int], error) {
		mu.Lock()
		calls[n]++
		mu.Unlock()
		return CalcRank(n, p, pos)
	}}
	ud := userdata{filterlocal: data}
	var queued []*Node[int]
	push := func(*Node[int], uint32) {}
	buffer := func(n *Node[int], _ interface{}, _ uint32) { queued = append(queued, n) }
	for _, leaf := range []*Node[int]{a, b} {
		if err := bottomUp(leaf, true, ud, push, buffer); err != nil {
			t.Fatal(err)
		}
	}
	if len(queued) != 2 {
		t.Fatalf("expected two completion notifications, got %d", len(queued))
	}
	var wg sync.WaitGroup
	for _, n := range queued {
		wg.Add(1)
		go func(n *Node[int]) { defer wg.Done(); _ = bottomUp(n, true, ud, push, buffer) }(n)
	}
	wg.Wait()
	if calls[root] != 1 || root.Rank != 3 {
		t.Fatalf("root calls=%d rank=%d", calls[root], root.Rank)
	}
}

func TestBottomUpCalcRankIntegration(t *testing.T) {
	root := NewNode(0)
	for i := 0; i < 8; i++ {
		branch := NewNode(i + 1)
		root.AddChild(branch)
		for j := 0; j < 8; j++ {
			branch.AddChild(NewNode(10 + i*8 + j))
		}
	}
	for pass := 0; pass < 3; pass++ {
		var mu sync.Mutex
		calls := map[*Node[int]]int{}
		action := func(n, p *Node[int], pos int) (*Node[int], error) {
			mu.Lock()
			calls[n]++
			mu.Unlock()
			return CalcRank(n, p, pos)
		}
		nodes, err := NewWalker(root).DescendentsWith(NodeIsLeaf[int]()).BottomUp(action).Promise()()
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 73 || len(calls) != 73 || root.Rank != 73 {
			t.Fatalf("nodes=%d calls=%d root rank=%d", len(nodes), len(calls), root.Rank)
		}
		for n, count := range calls {
			want := uint32(1)
			if n == root {
				want = 73
			} else if n.ChildCount() > 0 {
				want = 9
			}
			if count != 1 || n.Rank != want {
				t.Fatalf("node %v calls=%d rank=%d want=%d", n, count, n.Rank, want)
			}
		}
	}
}
