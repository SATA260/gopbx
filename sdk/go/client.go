// 这个文件定义 Go SDK 客户端骨架，给其他服务预留统一接入入口。

package gosdk

import "net/http"

type Client struct {
	Options    ClientOptions
	HTTPClient *http.Client
}

func NewClient(opts ClientOptions) *Client {
	return &Client{
		Options:    opts,
		HTTPClient: http.DefaultClient,
	}
}
