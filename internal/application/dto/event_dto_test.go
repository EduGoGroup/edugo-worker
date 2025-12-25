package dto

import (
	"testing"
	"time"
)

func TestMaterialUploadedEvent_GetMaterialID(t *testing.T) {
	event := MaterialUploadedEvent{
		EventID:      "evt-123",
		EventType:    "material.uploaded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: MaterialUploadedPayload{
			MaterialID:    "mat-456",
			SchoolID:      "school-789",
			TeacherID:     "teacher-101",
			FileURL:       "https://s3.amazonaws.com/bucket/materials/file.pdf",
			FileSizeBytes: 1024000,
			FileType:      "application/pdf",
		},
	}

	got := event.GetMaterialID()
	want := "mat-456"

	if got != want {
		t.Errorf("GetMaterialID() = %v, want %v", got, want)
	}
}

func TestMaterialUploadedEvent_GetS3Key(t *testing.T) {
	tests := []struct {
		name     string
		event    MaterialUploadedEvent
		expected string
	}{
		{
			name: "retorna FileURL directamente",
			event: MaterialUploadedEvent{
				EventID:      "evt-123",
				EventType:    "material.uploaded",
				EventVersion: "1.0.0",
				Timestamp:    time.Now(),
				Payload: MaterialUploadedPayload{
					MaterialID:    "mat-456",
					SchoolID:      "school-789",
					TeacherID:     "teacher-101",
					FileURL:       "https://s3.amazonaws.com/bucket/materials/file.pdf",
					FileSizeBytes: 1024000,
					FileType:      "application/pdf",
				},
			},
			expected: "https://s3.amazonaws.com/bucket/materials/file.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.GetS3Key()
			if got != tt.expected {
				t.Errorf("GetS3Key() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMaterialUploadedEvent_GetAuthorID(t *testing.T) {
	event := MaterialUploadedEvent{
		EventID:      "evt-123",
		EventType:    "material.uploaded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: MaterialUploadedPayload{
			MaterialID:    "mat-456",
			SchoolID:      "school-789",
			TeacherID:     "teacher-101",
			FileURL:       "https://s3.amazonaws.com/bucket/materials/file.pdf",
			FileSizeBytes: 1024000,
			FileType:      "application/pdf",
		},
	}

	got := event.GetAuthorID()
	want := "teacher-101"

	if got != want {
		t.Errorf("GetAuthorID() = %v, want %v", got, want)
	}
}

func TestMaterialUploadedPayload_Metadata(t *testing.T) {
	payload := MaterialUploadedPayload{
		MaterialID:    "mat-456",
		SchoolID:      "school-789",
		TeacherID:     "teacher-101",
		FileURL:       "https://s3.amazonaws.com/bucket/materials/file.pdf",
		FileSizeBytes: 1024000,
		FileType:      "application/pdf",
		Metadata: map[string]interface{}{
			"s3_key":        "materials/school-789/mat-456.pdf",
			"upload_source": "web",
		},
	}

	if payload.Metadata == nil {
		t.Error("Metadata no debe ser nil")
	}

	if len(payload.Metadata) != 2 {
		t.Errorf("Metadata debe tener 2 elementos, tiene %d", len(payload.Metadata))
	}

	s3Key, ok := payload.Metadata["s3_key"].(string)
	if !ok || s3Key != "materials/school-789/mat-456.pdf" {
		t.Errorf("s3_key en metadata = %v, want materials/school-789/mat-456.pdf", s3Key)
	}
}
