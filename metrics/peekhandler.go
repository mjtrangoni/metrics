/*
	Copyright NetFoundry, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package metrics

import (
	"github.com/michaelquigley/pfxlog"
	"github.com/netfoundry/ziti-fabric/router/xgress"
	"github.com/netfoundry/ziti-foundation/channel2"
	"time"
)

// NewChannelPeekHandler creates a channel PeekHandler which tracks latency, message rate and message size distribution
func NewChannelPeekHandler(linkId string, registry Registry) channel2.PeekHandler {
	appTxBytesMeter := registry.Meter("fabric.tx.bytesrate")
	appTxMsgMeter := registry.Meter("fabric.tx.msgrate")
	appTxMsgSizeHistogram := registry.Histogram("fabric.tx.msgsize")

	appRxBytesMeter := registry.Meter("fabric.rx.bytesrate")
	appRxMsgMeter := registry.Meter("fabric.rx.msgrate")
	appRxMsgSizeHistogram := registry.Histogram("fabric.rx.msgsize")

	linkTxBytesMeter := registry.Meter("link." + linkId + ".tx.bytesrate")
	linkTxMsgMeter := registry.Meter("link." + linkId + ".tx.msgrate")
	linkTxMsgSizeHistogram := registry.Histogram("link." + linkId + ".tx.msgsize")
	linkRxBytesMeter := registry.Meter("link." + linkId + ".rx.bytesrate")
	linkRxMsgMeter := registry.Meter("link." + linkId + ".rx.msgrate")
	linkRxMsgSizeHistogram := registry.Histogram("link." + linkId + ".rx.msgsize")

	usageRxCounter := registry.IntervalCounter("usage.fabric.rx", time.Minute)
	usageTxCounter := registry.IntervalCounter("usage.fabric.tx", time.Minute)

	closeHook := func() {
		linkTxBytesMeter.Dispose()
		linkTxMsgMeter.Dispose()
		linkTxMsgSizeHistogram.Dispose()
		linkRxBytesMeter.Dispose()
		linkRxMsgMeter.Dispose()
		linkRxMsgSizeHistogram.Dispose()
		// app level metrics and usageCounter are shared across all links, so we don't dispose of them
	}

	return &channelPeekHandler{
		appTxBytesMeter:        appTxBytesMeter,
		appTxMsgMeter:          appTxMsgMeter,
		appTxMsgSizeHistogram:  appTxMsgSizeHistogram,
		appRxBytesMeter:        appRxBytesMeter,
		appRxMsgMeter:          appRxMsgMeter,
		appRxMsgSizeHistogram:  appRxMsgSizeHistogram,
		linkTxBytesMeter:       linkTxBytesMeter,
		linkTxMsgMeter:         linkTxMsgMeter,
		linkTxMsgSizeHistogram: linkTxMsgSizeHistogram,
		linkRxBytesMeter:       linkRxBytesMeter,
		linkRxMsgMeter:         linkRxMsgMeter,
		linkRxMsgSizeHistogram: linkRxMsgSizeHistogram,
		usageRxCounter:         usageRxCounter,
		usageTxCounter:         usageTxCounter,
		closeHook:              closeHook,
	}
}

type channelPeekHandler struct {
	appTxBytesMeter Meter
	appTxMsgMeter   Meter
	appRxBytesMeter Meter
	appRxMsgMeter   Meter

	appTxMsgSizeHistogram Histogram
	appRxMsgSizeHistogram Histogram

	linkLatencyHistogram   Histogram
	linkTxBytesMeter       Meter
	linkTxMsgMeter         Meter
	linkRxBytesMeter       Meter
	linkRxMsgMeter         Meter
	linkTxMsgSizeHistogram Histogram
	linkRxMsgSizeHistogram Histogram

	usageRxCounter IntervalCounter
	usageTxCounter IntervalCounter

	closeHook func()
}

func (h *channelPeekHandler) Connect(ch channel2.Channel, remoteAddress string) {
}

func (h *channelPeekHandler) Rx(msg *channel2.Message, ch channel2.Channel) {
	msgSize := int64(len(msg.Body))
	h.linkRxBytesMeter.Mark(msgSize)
	h.linkRxMsgMeter.Mark(1)
	h.linkRxMsgSizeHistogram.Update(msgSize)
	h.appRxBytesMeter.Mark(msgSize)
	h.appRxMsgMeter.Mark(1)
	h.appRxMsgSizeHistogram.Update(msgSize)

	if msg.ContentType == int32(xgress.ContentTypePayloadType) {
		if payload, err := xgress.UnmarshallPayload(msg); err != nil {
			pfxlog.Logger().Errorf("Failed to unmarshal payload. Error: %v", err)
		} else {
			h.usageRxCounter.Update(payload.SessionId, time.Now(), uint64(len(payload.Data)))
		}
	}
}

func (h *channelPeekHandler) Tx(msg *channel2.Message, ch channel2.Channel) {
	msgSize := int64(len(msg.Body))
	h.linkTxBytesMeter.Mark(msgSize)
	h.linkTxMsgMeter.Mark(1)
	h.linkTxMsgSizeHistogram.Update(msgSize)
	h.appTxBytesMeter.Mark(msgSize)
	h.appTxMsgMeter.Mark(1)
	h.appTxMsgSizeHistogram.Update(msgSize)

	if msg.ContentType == int32(xgress.ContentTypePayloadType) {
		if payload, err := xgress.UnmarshallPayload(msg); err != nil {
			pfxlog.Logger().Errorf("Failed to unmarshal payload. Error: %v", err)
		} else {
			h.usageTxCounter.Update(payload.SessionId, time.Now(), uint64(len(payload.Data)))
		}
	}
}

func (h *channelPeekHandler) Close(ch channel2.Channel) {
	if h.closeHook != nil {
		h.closeHook()
	}
}

// NewXgressPeekHandler creates an xgress PeekHandler which tracks message rates and histograms as well as usage
func NewXgressPeekHandler(registry Registry) xgress.PeekHandler {
	ingressTxBytesMeter := registry.Meter("ingress.tx.bytesrate")
	ingressTxMsgMeter := registry.Meter("ingress.tx.msgrate")
	ingressRxBytesMeter := registry.Meter("ingress.rx.bytesrate")
	ingressRxMsgMeter := registry.Meter("ingress.rx.msgrate")
	egressTxBytesMeter := registry.Meter("egress.tx.bytesrate")
	egressTxMsgMeter := registry.Meter("egress.tx.Msgrate")
	egressRxBytesMeter := registry.Meter("egress.rx.bytesrate")
	egressRxMsgMeter := registry.Meter("egress.rx.msgrate")

	ingressTxMsgSizeHistogram := registry.Histogram("ingress.tx.msgsize")
	ingressRxMsgSizeHistogram := registry.Histogram("ingress.rx.msgsize")
	egressTxMsgSizeHistogram := registry.Histogram("egress.tx.msgsize")
	egressRxMsgSizeHistogram := registry.Histogram("egress.rx.msgsize")

	return &xgressPeekHandler{
		ingressTxBytesMeter: ingressTxBytesMeter,
		ingressTxMsgMeter:   ingressTxMsgMeter,
		ingressRxBytesMeter: ingressRxBytesMeter,
		ingressRxMsgMeter:   ingressRxMsgMeter,
		egressTxBytesMeter:  egressTxBytesMeter,
		egressTxMsgMeter:    egressTxMsgMeter,
		egressRxBytesMeter:  egressRxBytesMeter,
		egressRxMsgMeter:    egressRxMsgMeter,

		ingressTxMsgSizeHistogram: ingressTxMsgSizeHistogram,
		ingressRxMsgSizeHistogram: ingressRxMsgSizeHistogram,
		egressTxMsgSizeHistogram:  egressTxMsgSizeHistogram,
		egressRxMsgSizeHistogram:  egressRxMsgSizeHistogram,

		ingressRxUsageCounter: registry.IntervalCounter("usage.ingress.rx", time.Minute),
		ingressTxUsageCounter: registry.IntervalCounter("usage.ingress.tx", time.Minute),
		egressRxUsageCounter:  registry.IntervalCounter("usage.egress.rx", time.Minute),
		egressTxUsageCounter:  registry.IntervalCounter("usage.egress.tx", time.Minute),
	}
}

type xgressPeekHandler struct {
	ingressTxBytesMeter Meter
	ingressTxMsgMeter   Meter
	ingressRxBytesMeter Meter
	ingressRxMsgMeter   Meter
	egressTxBytesMeter  Meter
	egressTxMsgMeter    Meter
	egressRxBytesMeter  Meter
	egressRxMsgMeter    Meter

	ingressTxMsgSizeHistogram Histogram
	ingressRxMsgSizeHistogram Histogram
	egressTxMsgSizeHistogram  Histogram
	egressRxMsgSizeHistogram  Histogram

	ingressRxUsageCounter IntervalCounter
	ingressTxUsageCounter IntervalCounter
	egressRxUsageCounter  IntervalCounter
	egressTxUsageCounter  IntervalCounter
}

func (handler *xgressPeekHandler) Rx(x *xgress.Xgress, payload *xgress.Payload) {
	msgSize := int64(len(payload.Data))
	if x.Originator() == xgress.Initiator {
		handler.ingressRxUsageCounter.Update(x.SessionId().Token, time.Now(), uint64(msgSize))
		handler.ingressRxMsgMeter.Mark(1)
		handler.ingressRxBytesMeter.Mark(msgSize)
		handler.ingressRxMsgSizeHistogram.Update(msgSize)
	} else {
		handler.egressRxUsageCounter.Update(x.SessionId().Token, time.Now(), uint64(msgSize))
		handler.egressRxMsgMeter.Mark(1)
		handler.egressRxBytesMeter.Mark(msgSize)
		handler.egressRxMsgSizeHistogram.Update(msgSize)
	}
}

func (handler *xgressPeekHandler) Tx(x *xgress.Xgress, payload *xgress.Payload) {
	msgSize := int64(len(payload.Data))
	if x.Originator() == xgress.Initiator {
		handler.ingressTxUsageCounter.Update(x.SessionId().Token, time.Now(), uint64(msgSize))
		handler.ingressTxMsgMeter.Mark(1)
		handler.ingressTxBytesMeter.Mark(msgSize)
		handler.ingressTxMsgSizeHistogram.Update(msgSize)
	} else {
		handler.egressTxUsageCounter.Update(x.SessionId().Token, time.Now(), uint64(msgSize))
		handler.egressTxMsgMeter.Mark(1)
		handler.egressTxBytesMeter.Mark(msgSize)
		handler.egressTxMsgSizeHistogram.Update(msgSize)
	}
}

func (handler *xgressPeekHandler) Close(x *xgress.Xgress) {
}
