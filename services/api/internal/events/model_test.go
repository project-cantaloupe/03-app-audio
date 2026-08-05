package events

import "testing"

func TestScanResultRejectsFailOpenStatus(t *testing.T) {
	result := ScanResult{
		SchemaVersion: 1,
		EventID:       "018f86f3-2a5a-7f67-a1af-12655a22694e",
		Bucket:        "cntlp-aws-quarantine",
		Key:           "incoming/audio/upload/source",
		VersionID:     "v1",
		Status:        "UNKNOWN",
	}
	if err := result.Validate(); err == nil {
		t.Fatal("expected unknown scan status to be rejected")
	}
}

func TestSuccessfulResultRequiresBothArtifacts(t *testing.T) {
	result := TranscodeResult{
		SchemaVersion: 1,
		EventID:       "018f86f3-2a5a-7f67-a1af-12655a22694e",
		JobID:         "018f86f3-2a5a-7f67-a1af-12655a22694f",
		AudioID:       "018f86f3-2a5a-7f67-a1af-12655a226950",
		Status:        "SUCCEEDED",
		Attempt:       1,
		DurationMS:    1000,
		Artifacts:     &Artifacts{Bucket: "cntlp-aws-transcode", PlaybackKey: "playback.mp3"},
	}
	if err := result.Validate(); err == nil {
		t.Fatal("expected missing waveform artifact to be rejected")
	}
}
