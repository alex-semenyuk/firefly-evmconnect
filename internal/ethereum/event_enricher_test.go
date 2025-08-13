// Copyright © 2025 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ethereum

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hyperledger/firefly-signer/pkg/abi"
	"github.com/hyperledger/firefly-signer/pkg/ethtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEventEnricher_FilterEnrichEthLog_BasicMatch(t *testing.T) {
	_, conn, mRPC, done := newTestConnector(t)
	defer done()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "net_version", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		*args[1].(*string) = "1"
	}).Maybe()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "eth_getTransactionByHash", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		bi := &txInfoJSONRPC{
			BlockNumber: ethtypes.NewHexInteger64(100),
		}
		*args[1].(**txInfoJSONRPC) = bi
	}).Maybe()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "eth_getBlockByHash", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		bi := &blockInfoJSONRPC{
			Number: ethtypes.NewHexInteger64(100),
		}
		*args[1].(**blockInfoJSONRPC) = bi
	}).Maybe()

	ee := &eventEnricher{
		connector:     conn,
		extractSigner: false,
	}

	// Prepare a dummy ABI event
	var eventABI *abi.Entry
	err := json.Unmarshal([]byte(`{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}`), &eventABI)
	assert.NoError(t, err)

	// Prepare a filter for the event
	topic0, err := eventABI.SignatureHashCtx(context.Background())
	assert.NoError(t, err)
	addr := ethtypes.MustNewAddress("0x112233445566778899aabbccddeeff0011223344")
	filter := &eventFilter{
		Topic0:  topic0,
		Address: addr,
		Event:   eventABI,
	}

	// Prepare a log that matches the filter
	log := &logJSONRPC{
		Address:          addr,
		Topics:           []ethtypes.HexBytes0xPrefix{topic0},
		Data:             []byte{},
		BlockNumber:      ethtypes.NewHexInteger64(100),
		TransactionIndex: ethtypes.NewHexInteger64(1),
		LogIndex:         ethtypes.NewHexInteger64(0),
		BlockHash:        ethtypes.HexBytes0xPrefix{},
	}

	ctx := context.Background()
	ev, matched, _, err := ee.filterEnrichEthLog(ctx, filter, []*abi.Entry{eventABI}, log)
	assert.NoError(t, err)
	assert.True(t, matched)
	assert.NotNil(t, ev)
	// Decoded may be false if no data, but should not error
}

func TestEventEnricher_FilterEnrichEthLog_TopicNoMatch(t *testing.T) {
	_, conn, _, done := newTestConnector(t)
	defer done()
	ee := &eventEnricher{
		connector:     conn,
		extractSigner: false,
	}

	var eventABI *abi.Entry
	err := json.Unmarshal([]byte(`{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}`), &eventABI)
	assert.NoError(t, err)

	topic0, err := eventABI.SignatureHashCtx(context.Background())
	assert.NoError(t, err)
	addr := ethtypes.MustNewAddress("0x112233445566778899aabbccddeeff0011223344")
	filter := &eventFilter{
		Topic0:  topic0,
		Address: addr,
		Event:   eventABI,
	}

	// Prepare a log with a different topic
	otherTopic := make([]byte, len(topic0))
	copy(otherTopic, topic0)
	otherTopic[0] ^= 0xFF // change first byte

	log := &logJSONRPC{
		Address:          addr,
		Topics:           []ethtypes.HexBytes0xPrefix{otherTopic},
		Data:             []byte{},
		BlockNumber:      ethtypes.NewHexInteger64(100),
		TransactionIndex: ethtypes.NewHexInteger64(1),
		LogIndex:         ethtypes.NewHexInteger64(0),
		BlockHash:        ethtypes.HexBytes0xPrefix{},
	}

	ctx := context.Background()
	ev, matched, decoded, err := ee.filterEnrichEthLog(ctx, filter, []*abi.Entry{eventABI}, log)
	assert.NoError(t, err)
	assert.False(t, matched)
	assert.Nil(t, ev)
	assert.False(t, decoded)
}

func TestEventEnricher_FilterEnrichEthLog_AddressNoMatch(t *testing.T) {
	_, conn, _, done := newTestConnector(t)
	defer done()
	ee := &eventEnricher{
		connector:     conn,
		extractSigner: false,
	}

	var eventABI *abi.Entry
	err := json.Unmarshal([]byte(`{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}`), &eventABI)
	assert.NoError(t, err)

	topic0, err := eventABI.SignatureHashCtx(context.Background())
	assert.NoError(t, err)
	addr := ethtypes.MustNewAddress("0x112233445566778899aabbccddeeff0011223344")
	filter := &eventFilter{
		Topic0:  topic0,
		Address: addr,
		Event:   eventABI,
	}

	// Prepare a log with a different address
	otherAddr := ethtypes.MustNewAddress("0x99887766554433221100ffeeddccbbaa99887766")

	log := &logJSONRPC{
		Address:          otherAddr,
		Topics:           []ethtypes.HexBytes0xPrefix{topic0},
		Data:             []byte{},
		BlockNumber:      ethtypes.NewHexInteger64(100),
		TransactionIndex: ethtypes.NewHexInteger64(1),
		LogIndex:         ethtypes.NewHexInteger64(0),
		BlockHash:        ethtypes.HexBytes0xPrefix{},
	}

	ctx := context.Background()
	ev, matched, decoded, err := ee.filterEnrichEthLog(ctx, filter, []*abi.Entry{eventABI}, log)
	assert.NoError(t, err)
	assert.False(t, matched)
	assert.Nil(t, ev)
	assert.False(t, decoded)
}

func TestEventEnricher_FilterEnrichEthLog_NoAddressFilter(t *testing.T) {
	_, conn, mRPC, done := newTestConnector(t)
	defer done()

	mRPC.On("CallRPC", mock.Anything, mock.Anything, "net_version", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		*args[1].(*string) = "1"
	}).Maybe()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "eth_getTransactionByHash", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		bi := &txInfoJSONRPC{
			BlockNumber: ethtypes.NewHexInteger64(100),
		}
		*args[1].(**txInfoJSONRPC) = bi
	}).Maybe()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "eth_getBlockByHash", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		bi := &blockInfoJSONRPC{
			Number: ethtypes.NewHexInteger64(100),
		}
		*args[1].(**blockInfoJSONRPC) = bi
	}).Maybe()

	ee := &eventEnricher{
		connector:     conn,
		extractSigner: false,
	}

	var eventABI *abi.Entry
	err := json.Unmarshal([]byte(`{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}`), &eventABI)
	assert.NoError(t, err)

	topic0, err := eventABI.SignatureHashCtx(context.Background())
	assert.NoError(t, err)
	// No address filter
	filter := &eventFilter{
		Topic0:  topic0,
		Address: nil,
		Event:   eventABI,
	}

	addr := ethtypes.MustNewAddress("0x112233445566778899aabbccddeeff0011223344")

	log := &logJSONRPC{
		Address:          addr,
		Topics:           []ethtypes.HexBytes0xPrefix{topic0},
		Data:             []byte{},
		BlockNumber:      ethtypes.NewHexInteger64(100),
		TransactionIndex: ethtypes.NewHexInteger64(1),
		LogIndex:         ethtypes.NewHexInteger64(0),
		BlockHash:        ethtypes.HexBytes0xPrefix{},
	}

	ctx := context.Background()
	ev, matched, _, err := ee.filterEnrichEthLog(ctx, filter, []*abi.Entry{eventABI}, log)
	assert.NoError(t, err)
	assert.True(t, matched)
	assert.NotNil(t, ev)
}

func TestEventEnricher_FilterEnrichEthLog_ChainIDNotSet(t *testing.T) {
	_, conn, mRPC, done := newTestConnector(t)
	defer done()

	mRPC.On("CallRPC", mock.Anything, mock.Anything, "net_version", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		*args[1].(*string) = "1"
	}).Maybe()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "eth_getTransactionByHash", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		bi := &txInfoJSONRPC{
			BlockNumber: ethtypes.NewHexInteger64(100),
		}
		*args[1].(**txInfoJSONRPC) = bi
	}).Maybe()
	mRPC.On("CallRPC", mock.Anything, mock.Anything, "eth_getBlockByHash", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		bi := &blockInfoJSONRPC{
			Number: ethtypes.NewHexInteger64(100),
		}
		*args[1].(**blockInfoJSONRPC) = bi
	}).Maybe()

	// Unset chainID to force IsReady call
	conn.chainID = ""
	ee := &eventEnricher{
		connector:     conn,
		extractSigner: false,
	}

	var eventABI *abi.Entry
	err := json.Unmarshal([]byte(`{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}`), &eventABI)
	assert.NoError(t, err)

	topic0, err := eventABI.SignatureHashCtx(context.Background())
	assert.NoError(t, err)
	addr := ethtypes.MustNewAddress("0x112233445566778899aabbccddeeff0011223344")
	filter := &eventFilter{
		Topic0:  topic0,
		Address: addr,
		Event:   eventABI,
	}

	log := &logJSONRPC{
		Address:          addr,
		Topics:           []ethtypes.HexBytes0xPrefix{topic0},
		Data:             []byte{},
		BlockNumber:      ethtypes.NewHexInteger64(100),
		TransactionIndex: ethtypes.NewHexInteger64(1),
		LogIndex:         ethtypes.NewHexInteger64(0),
		BlockHash:        ethtypes.HexBytes0xPrefix{},
	}

	ctx := context.Background()
	ev, matched, _, err := ee.filterEnrichEthLog(ctx, filter, []*abi.Entry{eventABI}, log)
	assert.NoError(t, err)
	assert.True(t, matched)
	assert.NotNil(t, ev)
}
