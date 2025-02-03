package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/deso-protocol/core/lib"
	"github.com/deso-protocol/state-consumer/consumer"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// WebHandler is a handler for sending blockchain entries over HTTP or WebSocket.
type WebHandler struct {
	// EndpointURL is the URL to which JSON data will be sent via HTTP POST.
	EndpointURL string

	// UseWebSocket determines whether data should be sent via WebSocket.
	UseWebSocket bool
	// WSURL is the URL used for the WebSocket connection.
	WSURL string

	// wsConn holds the WebSocket connection once it is established.
	wsConn *websocket.Conn

	// MinBlockHeight is the minimum block height required before sending any data.
	MinBlockHeight uint64
}

// NewWebHandler returns a new instance of WebHandler.
// The minBlockHeight parameter specifies the minimum block height from which data should be sent.
func NewWebHandler(endpointURL string, useWebSocket bool, wsURL string, minBlockHeight uint64) *WebHandler {
	return &WebHandler{
		EndpointURL:    endpointURL,
		UseWebSocket:   useWebSocket,
		WSURL:          wsURL,
		MinBlockHeight: minBlockHeight,
	}
}

// No-op implementations for database/transaction related methods

func (wh *WebHandler) CommitTransaction() error {
	// No database used; nothing to commit.
	return nil
}

func (wh *WebHandler) GetParams() *lib.DeSoParams {
	// Return default parameters (or nil) if not used.
	return &lib.DeSoMainnetParams
}

func (wh *WebHandler) HandleSyncEvent(syncEvent consumer.SyncEvent) error {
	// No sync event handling needed for web-only flow.
	return nil
}

func (wh *WebHandler) InitiateTransaction() error {
	// No transaction to initiate.
	return nil
}

func (wh *WebHandler) RollbackTransaction() error {
	// Nothing to rollback.
	return nil
}

// HandleEntryBatch accepts a batch of StateChangeEntry items and sends them over the network.
// If the block height of the first entry is below MinBlockHeight, the batch is skipped.
func (wh *WebHandler) HandleEntryBatch(batchedEntries []*lib.StateChangeEntry) error {
	if len(batchedEntries) == 0 {
		return fmt.Errorf("WebHandler.HandleEntryBatch: no entries to send")
	}

	// Check block height: if the first entry is below the minimum threshold, skip sending.
	if batchedEntries[0].BlockHeight < wh.MinBlockHeight {
		return nil
	}

	// Send via HTTP if an endpoint URL is configured.
	if wh.EndpointURL != "" {
		return wh.pushBatchToEndpoint(batchedEntries)
	}

	// Otherwise, if WebSocket mode is enabled, send via WebSocket.
	if wh.UseWebSocket {
		return wh.sendBatchOverWebSocket(batchedEntries)
	}

	return fmt.Errorf("WebHandler.HandleEntryBatch: no endpoint configured")
}

// pushBatchToEndpoint marshals the batch of entries to JSON and sends them via an HTTP POST.
func (wh *WebHandler) pushBatchToEndpoint(batchedEntries []*lib.StateChangeEntry) error {
	jsonData, err := json.Marshal(batchedEntries)
	if err != nil {
		return errors.Wrap(err, "WebHandler.pushBatchToEndpoint: failed to marshal batch")
	}

	resp, err := http.Post(wh.EndpointURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrapf(err, "WebHandler.pushBatchToEndpoint: failed to send HTTP POST to %s", wh.EndpointURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("WebHandler.pushBatchToEndpoint: unexpected HTTP status code %d", resp.StatusCode)
	}

	return nil
}

// sendBatchOverWebSocket marshals the batch of entries to JSON and sends it over WebSocket.
func (wh *WebHandler) sendBatchOverWebSocket(batchedEntries []*lib.StateChangeEntry) error {
	// Establish a WebSocket connection if needed.
	if wh.wsConn == nil {
		var err error
		wh.wsConn, _, err = websocket.DefaultDialer.Dial(wh.WSURL, nil)
		if err != nil {
			return errors.Wrapf(err, "WebHandler.sendBatchOverWebSocket: failed to establish connection to %s", wh.WSURL)
		}
	}

	jsonData, err := json.Marshal(batchedEntries)
	if err != nil {
		return errors.Wrap(err, "WebHandler.sendBatchOverWebSocket: failed to marshal batch")
	}

	err = wh.wsConn.WriteMessage(websocket.TextMessage, jsonData)
	if err != nil {
		return errors.Wrap(err, "WebHandler.sendBatchOverWebSocket: failed to write websocket message")
	}

	return nil
}
