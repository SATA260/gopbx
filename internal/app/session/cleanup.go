// 这个文件提供会话清理入口，统一释放资源并移除已结束会话。

package session

// Cleanup 统一做终态收敛和管理器摘除。
// 业务侧可以在 defer 中无条件调用它，这样 hangup、kill、disconnect、error 都会走同一条收尾路径。
func Cleanup(m *Manager, s *Session, fallback CloseInfo) CloseInfo {
	if s == nil {
		return normalizeCloseInfo(fallback)
	}
	info := s.Finalize(fallback)
	m.Delete(s.ID)
	return info
}
