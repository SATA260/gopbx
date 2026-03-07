// 这个文件提供会话清理入口，统一释放资源并移除已结束会话。

package session

func Cleanup(m *Manager, s *Session, fallback CloseInfo) CloseInfo {
	if s == nil {
		return normalizeCloseInfo(fallback)
	}
	info := s.Finalize(fallback)
	m.Delete(s.ID)
	return info
}
