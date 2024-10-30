package oss

import (
	"fmt"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type Op int

const (
	Upload Op = iota
	Download
	Remove
	Copy
)

func (op Op) String() string {
	switch op {
	case Upload:
		return "UPLOAD"
	case Download:
		return "DOWNLOAD"
	case Remove:
		return "REMOVE"
	case Copy:
		return "COPY"
	}
	return "UNDEFINED"
}

type ProgressEvent struct {
	Src  string
	Dist string
	oss.ProgressEvent
	Op Op
}

func (e ProgressEvent) Status() string {
	switch e.EventType {
	// TransferStartedEvent transfer started, set TotalBytes
	case oss.TransferStartedEvent:
		return "Started"
		// TransferDataEvent transfer data, set ConsumedBytes and TotalBytes
	case oss.TransferDataEvent:
		return "Transfering"
		// TransferCompletedEvent transfer completed
	case oss.TransferCompletedEvent:
		return "Completed"
		// TransferFailedEvent transfer encounters an error
	case oss.TransferFailedEvent:
		return "Failed"
	}
	return "Unknown"
}

func (e ProgressEvent) Progress() float64 {
	if e.TotalBytes == 0 {
		return 0
	}
	return float64(e.ConsumedBytes) * 100 / float64(e.TotalBytes)
}

func (e ProgressEvent) String() string {
	var prefix string
	if e.EventType == oss.TransferDataEvent {
		prefix = fmt.Sprintf("[%s(%.2f%%)] %s", e.Op, e.Progress(), e.Status())
	} else {
		prefix = fmt.Sprintf("[%s] %s", e.Op, e.Status())
	}
	switch e.Op {
	case Remove:
		return fmt.Sprintf("%s, %s, totalBytes:%d, consumedBytes:%d, RwBytes:%d", prefix, e.Src, e.TotalBytes, e.ConsumedBytes, e.RwBytes)
	case Upload:
		return fmt.Sprintf("%s, local:%s, remote:%s, totalBytes:%d, consumedBytes:%d, RwBytes:%d", prefix, e.Src, e.Dist, e.TotalBytes, e.ConsumedBytes, e.RwBytes)
	case Download:
		return fmt.Sprintf("%s, remote:%s, local:%s, totalBytes:%d, consumedBytes:%d, RwBytes:%d", prefix, e.Src, e.Dist, e.TotalBytes, e.ConsumedBytes, e.RwBytes)
	case Copy:
		return fmt.Sprintf("%s, from:%s, to:%s, totalBytes:%d, consumedBytes:%d, RwBytes:%d", prefix, e.Src, e.Dist, e.TotalBytes, e.ConsumedBytes, e.RwBytes)
	}
	return ""
}

// 定义进度条监听器。
type ProgressListener struct {
	ch   chan<- ProgressEvent
	src  string
	dist string
	op   Op
}

type listenerOpt func(*ProgressListener)

func WithProgressListenerSrc(src string) listenerOpt {
	return func(l *ProgressListener) {
		l.src = src
	}
}

func WithProgressListenerDist(dist string) listenerOpt {
	return func(l *ProgressListener) {
		l.dist = dist
	}
}

func WithProgressOp(op Op) listenerOpt {
	return func(l *ProgressListener) {
		l.op = op
	}
}

func NewProgressListener(ch chan<- ProgressEvent, opts ...listenerOpt) *ProgressListener {
	ret := &ProgressListener{
		ch: ch,
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// 定义进度变更事件处理函数。
func (listener *ProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	listener.ch <- ProgressEvent{
		ProgressEvent: *event,
		Op:            listener.op,
		Src:           listener.src,
		Dist:          listener.dist,
	}
}

type MultiProgressListener struct {
	ch chan ProgressEvent
}

func NewMultiProgressListener() *MultiProgressListener {
	ch := make(chan ProgressEvent, 10000)
	return &MultiProgressListener{
		ch: ch,
	}
}

func (listener *MultiProgressListener) DownloadListener(remote string, local string) oss.ProgressListener {
	return NewProgressListener(listener.ch, WithProgressOp(Download), WithProgressListenerSrc(remote), WithProgressListenerDist(local))
}

func (listener *MultiProgressListener) UploadListener(local string, remote string) oss.ProgressListener {
	return NewProgressListener(listener.ch, WithProgressOp(Upload), WithProgressListenerDist(remote), WithProgressListenerSrc(local))
}

func (listener *MultiProgressListener) CopyListener(src string, dist string) oss.ProgressListener {
	return NewProgressListener(listener.ch, WithProgressOp(Upload), WithProgressListenerSrc(src), WithProgressListenerDist(dist))
}

func (listener *MultiProgressListener) RemoveListener(src string) oss.ProgressListener {
	return NewProgressListener(listener.ch, WithProgressOp(Remove), WithProgressListenerSrc(src))
}

func (listener *MultiProgressListener) Events() <-chan ProgressEvent {
	return listener.ch
}

func (listener *MultiProgressListener) Close() {
	close(listener.ch)
}
