package collector

import (
	"context"
	"sync"
	"testing"
	"time"

	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

type fakeGroup struct {
	name  string
	call  func(context.Context)
	group GroupResult
}

func (f fakeGroup) Name() string { return f.name }
func (f fakeGroup) Collect(ctx context.Context, _ *reportingpb.ReportWindowSet) GroupResult {
	if f.call != nil {
		f.call(ctx)
	}
	return f.group
}

func TestCollectorRunsAllGroupsConcurrentlyAndPreservesFailures(t *testing.T) {
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var once sync.Once
	groups := make([]GroupClient, 0, 4)
	for _, name := range []string{"auth", "expense", "finance", "datarights"} {
		name := name
		groups = append(groups, fakeGroup{
			name: name,
			call: func(context.Context) {
				started <- struct{}{}
				once.Do(func() {
					for range 4 {
						<-started
					}
					close(release)
				})
				<-release
			},
			group: GroupResult{Name: name},
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	collection := (Collector{Clients: groups, Timeout: time.Second}).Collect(ctx, nil)
	if len(collection.Groups) != 4 {
		t.Fatalf("groups = %d, want 4", len(collection.Groups))
	}
	for index, name := range []string{"auth", "datarights", "expense", "finance"} {
		if collection.Groups[index].Name != name {
			t.Fatalf("group %d = %q, want %q", index, collection.Groups[index].Name, name)
		}
	}
}

func TestCollectorRecoversClientPanic(t *testing.T) {
	groups := []GroupClient{
		fakeGroup{name: "auth", call: func(context.Context) { panic("test panic") }},
	}
	collection := (Collector{Clients: groups, Timeout: time.Second}).Collect(context.Background(), nil)
	if len(collection.Groups) != 1 || collection.Groups[0].Err == nil {
		t.Fatalf("collection = %+v, want one failed group", collection)
	}
	if len(collection.Groups[0].Metrics) != 2 {
		t.Fatalf("metrics = %d, want auth metrics", len(collection.Groups[0].Metrics))
	}
}

func TestCollectorStopsReceivingAfterCancellationAndKeepsCompletedResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan struct{})
	release := make(chan struct{})
	groups := []GroupClient{
		fakeGroup{name: "auth", call: func(context.Context) {
			close(completed)
		}, group: GroupResult{Name: "auth"}},
		fakeGroup{name: "expense", call: func(context.Context) {
			<-release
		}, group: GroupResult{Name: "expense"}},
	}
	collectionDone := make(chan Collection, 1)
	go func() {
		collectionDone <- (Collector{Clients: groups, Timeout: time.Second}).Collect(ctx, nil)
	}()
	<-completed
	cancel()
	select {
	case collection := <-collectionDone:
		if group, ok := collection.Group("auth"); !ok || group.Err != nil {
			t.Fatalf("completed group = %+v, want successful result", group)
		}
		if group, ok := collection.Group("expense"); !ok || group.Err == nil {
			t.Fatalf("canceled group = %+v, want failure", group)
		}
	case <-time.After(time.Second):
		t.Fatal("collector did not stop after context cancellation")
	}
	close(release)
}

func TestCollectorDeadlineReachesEachGroup(t *testing.T) {
	groups := make([]GroupClient, 0, 4)
	for _, name := range []string{"auth", "expense", "finance", "datarights"} {
		groups = append(groups, fakeGroup{name: name, call: func(ctx context.Context) {
			<-ctx.Done()
		}})
	}
	started := time.Now()
	_ = (Collector{Clients: groups, Timeout: 5 * time.Millisecond}).Collect(context.Background(), nil)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("collector took %s", elapsed)
	}
}
