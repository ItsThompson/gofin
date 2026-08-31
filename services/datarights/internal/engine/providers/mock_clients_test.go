package providers

import (
	"context"
	"io"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// mockExpenseServiceClient implements ExpenseServiceClient for tests. The
// expenses provider consumes StreamAllUserExpenses, so streamRows seeds the
// server stream (one ExpenseData per Recv, in order). streamOpenErr fails the
// RPC open; recvErr fails a Recv (at recvErrAt, 1-based; 0 = after all rows).
type mockExpenseServiceClient struct {
	streamRows    []*expensepb.ExpenseData
	streamOpenErr error
	recvErr       error
	recvErrAt     int
	lastStreamReq *expensepb.StreamAllUserExpensesRequest
	callCount     int
}

// fakeExpenseStream is the client side of a StreamAllUserExpenses server stream.
// The embedded nil grpc.ClientStream supplies the ClientStream methods the
// consumer never calls; only Recv is exercised.
type fakeExpenseStream struct {
	grpc.ClientStream
	rows      []*expensepb.ExpenseData
	idx       int
	recvErr   error
	recvErrAt int
}

func (f *fakeExpenseStream) Recv() (*expensepb.ExpenseData, error) {
	if f.recvErr != nil && f.recvErrAt > 0 && f.idx+1 == f.recvErrAt {
		return nil, f.recvErr
	}
	if f.idx >= len(f.rows) {
		if f.recvErr != nil && f.recvErrAt == 0 {
			return nil, f.recvErr
		}
		return nil, io.EOF
	}
	row := f.rows[f.idx]
	f.idx++
	return row, nil
}

func (m *mockExpenseServiceClient) StreamAllUserExpenses(_ context.Context, req *expensepb.StreamAllUserExpensesRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[expensepb.ExpenseData], error) {
	m.callCount++
	m.lastStreamReq = req
	if m.streamOpenErr != nil {
		return nil, m.streamOpenErr
	}
	return &fakeExpenseStream{rows: m.streamRows, recvErr: m.recvErr, recvErrAt: m.recvErrAt}, nil
}

// Implement remaining interface methods as no-ops.
func (m *mockExpenseServiceClient) CreateExpense(_ context.Context, _ *expensepb.CreateExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetExpensesForPeriod(_ context.Context, _ *expensepb.GetExpensesForPeriodRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetExpense(_ context.Context, _ *expensepb.GetExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) CountExpensesByTag(_ context.Context, _ *expensepb.CountExpensesByTagRequest, _ ...grpc.CallOption) (*expensepb.CountExpensesByTagResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) CorrectExpense(_ context.Context, _ *expensepb.CorrectExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) CreateProRataInstallment(_ context.Context, _ *expensepb.CreateProRataInstallmentRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetCorrectionHistory(_ context.Context, _ *expensepb.GetCorrectionHistoryRequest, _ ...grpc.CallOption) (*expensepb.CorrectionHistoryResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetProRataGroup(_ context.Context, _ *expensepb.GetProRataGroupRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) AnonymizeAllUserExpenses(_ context.Context, _ *expensepb.AnonymizeRequest, _ ...grpc.CallOption) (*expensepb.AnonymizeResponse, error) {
	return nil, nil
}
