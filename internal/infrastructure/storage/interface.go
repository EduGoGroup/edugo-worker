package storage

import sharedstorage "github.com/EduGoGroup/edugo-shared/storage"

// Client es un alias del shared storage.Client.
// Permite que el código existente (mocks, circuit breaker, processors)
// siga usando storage.Client sin cambiar imports.
type Client = sharedstorage.Client

// FileMetadata es un alias del shared storage.FileMetadata.
type FileMetadata = sharedstorage.FileMetadata
