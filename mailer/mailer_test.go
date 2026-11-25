package mailer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"reflect"
	"syscall"
	"testing"
)

func generateMessages(dialer Dialer) []Mail {
	to := []string{"to@example.com"}

	messageContents := []io.WriterTo{
		bytes.NewBuffer([]byte("First email")),
		bytes.NewBuffer([]byte("Second email")),
	}

	m1 := newMockMessage("first@example.com", to, messageContents[0])
	m2 := newMockMessage("second@example.com", to, messageContents[1])

	m1.setDialer(func() (Dialer, error) { return dialer, nil })

	messages := []Mail{m1, m2}
	return messages
}

func newMockErrorSender(err error) *mockSender {
	sender := newMockSender()
	// The sending function will send a temporary error to emulate
	// a backoff.
	sender.setSend(func(mm *mockMessage) error {
		if len(sender.messages) == 1 {
			return err
		}
		sender.messageChan <- mm
		return nil
	})
	return sender
}

func TestDialHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	md := newMockDialer()
	md.setDial(md.unreachableDial)
	_, err := dialHost(ctx, md)
	if _, ok := err.(*ErrMaxConnectAttempts); !ok {
		t.Fatalf("Didn't receive expected ErrMaxConnectAttempts. Got: %s", err)
	}
	e := err.(*ErrMaxConnectAttempts)
	if e.underlyingError != errHostUnreachable {
		t.Fatalf("Got invalid underlying error. Expected %s Got %s\n", e.underlyingError, errHostUnreachable)
	}
	if md.dialCount != MaxReconnectAttempts {
		t.Fatalf("Unexpected number of reconnect attempts. Expected %d, Got %d", MaxReconnectAttempts, md.dialCount)
	}
	md.setDial(md.defaultDial)
	_, err = dialHost(ctx, md)
	if err != nil {
		t.Fatalf("Unexpected error when dialing the mock host: %s", err)
	}
}

func TestMailWorkerStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mw := NewMailWorker()
	go func(ctx context.Context) {
		mw.Start(ctx)
	}(ctx)

	sender := newMockSender()
	dialer := newMockDialer()
	dialer.setDial(func() (Sender, error) {
		return sender, nil
	})

	messages := generateMessages(dialer)

	// Send the campaign
	mw.Queue(messages)

	got := []*mockMessage{}

	idx := 0
	for message := range sender.messageChan {
		got = append(got, message)
		original := messages[idx].(*mockMessage)
		if original.from != message.from {
			t.Fatalf("Invalid message received. Expected %s, Got %s", original.from, message.from)
		}
		idx++
	}
	if len(got) != len(messages) {
		t.Fatalf("Unexpected number of messages received. Expected %d Got %d", len(got), len(messages))
	}
}

func TestBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mw := NewMailWorker()
	go func(ctx context.Context) {
		mw.Start(ctx)
	}(ctx)

	expectedError := &textproto.Error{
		Code: 400,
		Msg:  "Temporary error",
	}

	sender := newMockErrorSender(expectedError)
	dialer := newMockDialer()
	dialer.setDial(func() (Sender, error) {
		return sender, nil
	})

	messages := generateMessages(dialer)

	// Send the campaign
	mw.Queue(messages)

	got := []*mockMessage{}

	for message := range sender.messageChan {
		got = append(got, message)
	}
	// Check that we only sent one message
	expectedCount := 1
	if len(got) != expectedCount {
		t.Fatalf("Unexpected number of messages received. Expected %d Got %d", len(got), expectedCount)
	}

	// Check that it's the correct message
	originalFrom := messages[1].(*mockMessage).from
	if got[0].from != originalFrom {
		t.Fatalf("Invalid message received. Expected %s, Got %s", originalFrom, got[0].from)
	}

	// Check that the first message performed a backoff
	backoffCount := messages[0].(*mockMessage).backoffCount
	if backoffCount != expectedCount {
		t.Fatalf("Did not receive expected backoff. Got backoffCount %d, Expected %d", backoffCount, expectedCount)
	}

	// Check that there was a reset performed on the sender
	if sender.resetCount != expectedCount {
		t.Fatalf("Did not receive expected reset. Got resetCount %d, expected %d", sender.resetCount, expectedCount)
	}
}

func TestPermError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mw := NewMailWorker()
	go func(ctx context.Context) {
		mw.Start(ctx)
	}(ctx)

	expectedError := &textproto.Error{
		Code: 500,
		Msg:  "Permanent error",
	}

	sender := newMockErrorSender(expectedError)
	dialer := newMockDialer()
	dialer.setDial(func() (Sender, error) {
		return sender, nil
	})

	messages := generateMessages(dialer)

	// Send the campaign
	mw.Queue(messages)

	got := []*mockMessage{}

	for message := range sender.messageChan {
		got = append(got, message)
	}
	// Check that we only sent one message
	expectedCount := 1
	if len(got) != expectedCount {
		t.Fatalf("Unexpected number of messages received. Expected %d Got %d", len(got), expectedCount)
	}

	// Check that it's the correct message
	originalFrom := messages[1].(*mockMessage).from
	if got[0].from != originalFrom {
		t.Fatalf("Invalid message received. Expected %s, Got %s", originalFrom, got[0].from)
	}

	message := messages[0].(*mockMessage)

	// Check that the first message did not perform a backoff
	expectedBackoffCount := 0
	backoffCount := message.backoffCount
	if backoffCount != expectedBackoffCount {
		t.Fatalf("Did not receive expected backoff. Got backoffCount %d, Expected %d", backoffCount, expectedCount)
	}

	// Check that there was a reset performed on the sender
	if sender.resetCount != expectedCount {
		t.Fatalf("Did not receive expected reset. Got resetCount %d, expected %d", sender.resetCount, expectedCount)
	}

	// Check that the email errored out appropriately
	if !reflect.DeepEqual(message.err, expectedError) {
		t.Fatalf("Did not received expected error. Got %#v\nExpected %#v", message.err, expectedError)
	}
}

func TestUnknownError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mw := NewMailWorker()
	go func(ctx context.Context) {
		mw.Start(ctx)
	}(ctx)

	expectedError := errors.New("Unexpected error")

	sender := newMockErrorSender(expectedError)
	dialer := newMockDialer()
	dialer.setDial(func() (Sender, error) {
		return sender, nil
	})

	messages := generateMessages(dialer)

	// Send the campaign
	mw.Queue(messages)

	got := []*mockMessage{}

	for message := range sender.messageChan {
		got = append(got, message)
	}
	// Check that we only sent one message
	expectedCount := 1
	if len(got) != expectedCount {
		t.Fatalf("Unexpected number of messages received. Expected %d Got %d", len(got), expectedCount)
	}

	// Check that it's the correct message
	originalFrom := messages[1].(*mockMessage).from
	if got[0].from != originalFrom {
		t.Fatalf("Invalid message received. Expected %s, Got %s", originalFrom, got[0].from)
	}

	message := messages[0].(*mockMessage)

	// If we get an unexpected error, this means that it's likely the
	// underlying connection dropped. When this happens, we expect the
	// connection to be re-established (see #997).
	// In this case, we're successfully reestablishing the connection
	// so we expect the backoff to occur.
	expectedBackoffCount := 1
	backoffCount := message.backoffCount
	if backoffCount != expectedBackoffCount {
		t.Fatalf("Did not receive expected backoff. Got backoffCount %d, Expected %d", backoffCount, expectedBackoffCount)
	}

	// Check that the underlying connection was reestablished
	expectedDialCount := 2
	if dialer.dialCount != expectedDialCount {
		t.Fatalf("Did not receive expected dial count. Got %d expected %d", dialer.dialCount, expectedDialCount)
	}

	// Check that the email errored out appropriately
	if !reflect.DeepEqual(message.err, expectedError) {
		t.Fatalf("Did not received expected error. Got %#v\nExpected %#v", message.err, expectedError)
	}
}

type retryableError struct{ retryable bool }

func (e *retryableError) Error() string   { return "api error" }
func (e *retryableError) Retryable() bool { return e.retryable }

func runRetryableCase(t *testing.T, retryable bool) (*mockMessage, *mockSender, *mockDialer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mw := NewMailWorker()
	go mw.Start(ctx)

	expectedError := fmt.Errorf("wrapped: %w", &retryableError{retryable: retryable})

	sender := newMockErrorSender(expectedError)
	dialer := newMockDialer()
	dialer.setDial(func() (Sender, error) { return sender, nil })

	messages := generateMessages(dialer)
	mw.Queue(messages)

	got := []*mockMessage{}
	for message := range sender.messageChan {
		got = append(got, message)
	}
	if len(got) != 1 || got[0].from != messages[1].(*mockMessage).from {
		t.Fatalf("Expected only the second message to be sent, got %d messages", len(got))
	}
	return messages[0].(*mockMessage), sender, dialer, expectedError
}

func TestRetryableErrorBacksOff(t *testing.T) {
	message, sender, dialer, expectedError := runRetryableCase(t, true)

	if message.backoffCount != 1 {
		t.Fatalf("Expected backoffCount 1, got %d", message.backoffCount)
	}
	if message.finished {
		t.Fatal("Retryable error should not finish the message")
	}
	if message.err != expectedError {
		t.Fatalf("Unexpected error recorded: %v", message.err)
	}
	if sender.resetCount != 0 {
		t.Fatalf("Sender should not be reset, got resetCount %d", sender.resetCount)
	}
	if dialer.dialCount != 1 {
		t.Fatalf("Sender should not be redialled, got dialCount %d", dialer.dialCount)
	}
}

func TestNonRetryableErrorFails(t *testing.T) {
	message, sender, dialer, expectedError := runRetryableCase(t, false)

	if message.backoffCount != 0 {
		t.Fatalf("Expected no backoff, got %d", message.backoffCount)
	}
	if !message.finished || message.err != expectedError {
		t.Fatalf("Message should error out with the original error, got %v", message.err)
	}
	if sender.resetCount != 0 {
		t.Fatalf("Sender should not be reset, got resetCount %d", sender.resetCount)
	}
	if dialer.dialCount != 1 {
		t.Fatalf("Sender should not be redialled, got dialCount %d", dialer.dialCount)
	}
}

func TestNetOpErrorRedials(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mw := NewMailWorker()
	go mw.Start(ctx)

	opErr := &net.OpError{Op: "write", Net: "tcp", Err: syscall.ECONNRESET}
	sender := newMockErrorSender(opErr)
	dialer := newMockDialer()
	dialer.setDial(func() (Sender, error) { return sender, nil })

	messages := generateMessages(dialer)
	mw.Queue(messages)

	got := []*mockMessage{}
	for message := range sender.messageChan {
		got = append(got, message)
	}
	if len(got) != 1 || got[0].from != messages[1].(*mockMessage).from {
		t.Fatalf("Expected only the second message to be sent, got %d messages", len(got))
	}
	message := messages[0].(*mockMessage)
	if message.backoffCount != 1 || message.finished {
		t.Fatalf("Expected a single backoff, got backoffCount %d finished %v", message.backoffCount, message.finished)
	}
	if dialer.dialCount != 2 {
		t.Fatalf("Expected the sender to be redialled, got dialCount %d", dialer.dialCount)
	}
}
