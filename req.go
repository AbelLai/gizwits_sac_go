package gizwits_sac_go

type LoginReq struct {
	Cmd           string     `json:"cmd"`
	PrefetchCount int        `json:"prefetch_count"`
	Data          []AuthData `json:"data"`
}

type T interface{}

type AttrsRCtrlReq struct {
	Cmd   string              `json:"cmd"`
	MsgId string              `json:"msg_id"`
	Items []AttrsRCtrlReqItem `json:"data"`
}

type AttrsRCtrlReqItem struct {
	Cmd  string                `json:"cmd"`
	Data AttrsRCtrlReqItemData `json:"data"`
}

type AttrsRCtrlReqItemData struct {
	Did        string                 `json:"did"`
	Mac        string                 `json:"mac"`
	ProductKey string                 `json:"product_key"`
	Attrs      map[string]interface{} `json:"attrs"`
}

type RCtrlReq struct {
	Cmd   string         `json:"cmd"`
	MsgId string         `json:"msg_id"`
	Items []RCtrlReqItem `json:"data"`
}

type RCtrlReqItem struct {
	Cmd  string           `json:"cmd"`
	Data RCtrlReqItemData `json:"data"`
}

type RCtrlReqItemData struct {
	Did        string `json:"did"`
	Mac        string `json:"mac"`
	ProductKey string `json:"product_key"`
	Raw        []byte `json:"raw"`
}
