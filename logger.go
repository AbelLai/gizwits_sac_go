package gizwits_sac_go

type ILogger interface {
	Error(string)
	Warn(string)
	Info(string)
}
