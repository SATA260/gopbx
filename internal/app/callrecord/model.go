// 这个文件定义话单模型，描述一次会话结束后需要归档的核心元数据。

package callrecord

import "time"

type Record struct {
	CallType        string    `json:"callType"`
	CallID          string    `json:"callId"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	Caller          string    `json:"caller,omitempty"`
	Callee          string    `json:"callee,omitempty"`
	Offer           string    `json:"offer,omitempty"`
	Answer          string    `json:"answer,omitempty"`
	HangupReason    string    `json:"hangupReason,omitempty"`
	HangupInitiator string    `json:"hangupInitiator,omitempty"`
	Error           string    `json:"error,omitempty"`
	Commands        []string  `json:"commands,omitempty"`
	DumpEventFile   string    `json:"dumpEventFile,omitempty"`
}
