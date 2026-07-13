package nginx

import (
	"github.com/Rain-kl/Wavelet/internal/apps/agent/runtimeuser"
)

// OpenFlareRuntimeUser is the shared OS account for the agent process and
// OpenResty worker processes.
const OpenFlareRuntimeUser = runtimeuser.Name

// OpenRestyWorkerUser is an alias kept for internal call sites.
const OpenRestyWorkerUser = runtimeuser.Name

// EnsureWorldTraversablePath makes targetDir and its ancestors world-traversable.
func EnsureWorldTraversablePath(targetDir string) error {
	return runtimeuser.EnsurePathOwnership(targetDir, nginxDirPerm, nginxConfigFilePerm)
}

// EnsureWorkerReadableTree normalizes ownership and modes under root for the
// shared runtime user.
func EnsureWorkerReadableTree(rootDir string) error {
	return runtimeuser.EnsurePathOwnership(rootDir, nginxDirPerm, nginxConfigFilePerm)
}

// EnsureWorkerReadAccess makes agent-managed runtime paths accessible to the
// shared runtime user.
func (m *Manager) EnsureWorkerReadAccess() error {
	return m.ensureOpenRestyWorkerReadAccess()
}
