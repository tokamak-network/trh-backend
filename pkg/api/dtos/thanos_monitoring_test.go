package dtos

import (
	"encoding/json"
	"testing"
)

func TestTelegramReceiverJSONTag(t *testing.T) {
	t.Run("marshal uses camelCase chatId", func(t *testing.T) {
		receiver := TelegramReceiver{ChatId: "123456"}
		data, err := json.Marshal(receiver)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}
		expected := `{"chatId":"123456"}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("unmarshal from camelCase chatId", func(t *testing.T) {
		input := `{"chatId":"789"}`
		var receiver TelegramReceiver
		if err := json.Unmarshal([]byte(input), &receiver); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if receiver.ChatId != "789" {
			t.Errorf("got %s, want 789", receiver.ChatId)
		}
	})

	t.Run("unmarshal from PascalCase ChatId (backward compat)", func(t *testing.T) {
		input := `{"ChatId":"789"}`
		var receiver TelegramReceiver
		if err := json.Unmarshal([]byte(input), &receiver); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if receiver.ChatId != "789" {
			t.Errorf("got %s, want 789", receiver.ChatId)
		}
	})
}

func TestInstallMonitoringRequest_Validate(t *testing.T) {
	validRequest := func() InstallMonitoringRequest {
		return InstallMonitoringRequest{
			GrafanaPassword: "strongpassword",
			LoggingEnabled:  true,
			AlertManager: AlertManagerConfig{
				Telegram: TelegramConfig{
					Enabled: false,
				},
				Email: EmailConfig{
					Enabled: false,
				},
			},
		}
	}

	t.Run("valid request with alerts disabled", func(t *testing.T) {
		req := validRequest()
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty grafana password fails", func(t *testing.T) {
		req := validRequest()
		req.GrafanaPassword = ""
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty grafana password")
		}
	})

	t.Run("email enabled forces smtpSmarthost to gmail", func(t *testing.T) {
		req := validRequest()
		req.AlertManager.Email = EmailConfig{
			Enabled:          true,
			SmtpSmarthost:    "wrong-value",
			SmtpFrom:         "user@gmail.com",
			SmtpAuthPassword: "apppassword",
			AlertReceivers:   []string{"receiver@gmail.com"},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if req.AlertManager.Email.SmtpSmarthost != "smtp.gmail.com:587" {
			t.Errorf("got %s, want smtp.gmail.com:587", req.AlertManager.Email.SmtpSmarthost)
		}
	})

	t.Run("email enabled with empty smtpSmarthost defaults to gmail", func(t *testing.T) {
		req := validRequest()
		req.AlertManager.Email = EmailConfig{
			Enabled:          true,
			SmtpSmarthost:    "",
			SmtpFrom:         "user@gmail.com",
			SmtpAuthPassword: "apppassword",
			AlertReceivers:   []string{"receiver@gmail.com"},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if req.AlertManager.Email.SmtpSmarthost != "smtp.gmail.com:587" {
			t.Errorf("got %s, want smtp.gmail.com:587", req.AlertManager.Email.SmtpSmarthost)
		}
	})

	t.Run("telegram enabled with empty chatId fails", func(t *testing.T) {
		req := validRequest()
		req.AlertManager.Telegram = TelegramConfig{
			Enabled:           true,
			ApiToken:          "123456:ABCdef",
			CriticalReceivers: []TelegramReceiver{{ChatId: ""}},
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty chatId")
		}
	})

	t.Run("telegram enabled with valid config succeeds", func(t *testing.T) {
		req := validRequest()
		req.AlertManager.Telegram = TelegramConfig{
			Enabled:           true,
			ApiToken:          "123456:ABCdef",
			CriticalReceivers: []TelegramReceiver{{ChatId: "1266746900"}},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("email password whitespace is cleaned", func(t *testing.T) {
		req := validRequest()
		req.AlertManager.Email = EmailConfig{
			Enabled:          true,
			SmtpFrom:         "user@gmail.com",
			SmtpAuthPassword: "abcd efgh ijkl mnop",
			AlertReceivers:   []string{"receiver@gmail.com"},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if req.AlertManager.Email.SmtpAuthPassword == "abcd efgh ijkl mnop" {
			t.Error("expected password whitespace to be cleaned")
		}
	})
}

func TestInstallMonitoringRequest_ToSDKAlertManagerConfig(t *testing.T) {
	req := InstallMonitoringRequest{
		GrafanaPassword: "password",
		LoggingEnabled:  true,
		AlertManager: AlertManagerConfig{
			Telegram: TelegramConfig{
				Enabled:           true,
				ApiToken:          "token123:abc",
				CriticalReceivers: []TelegramReceiver{{ChatId: "111"}, {ChatId: "222"}},
			},
			Email: EmailConfig{
				Enabled:          true,
				SmtpSmarthost:    "smtp.gmail.com:587",
				SmtpFrom:         "test@gmail.com",
				SmtpAuthPassword: "pass",
				AlertReceivers:   []string{"a@b.com", "c@d.com"},
			},
		},
	}

	config := req.ToSDKAlertManagerConfig()

	// Telegram
	if !config.Telegram.Enabled {
		t.Error("expected telegram enabled")
	}
	if config.Telegram.ApiToken != "token123:abc" {
		t.Errorf("got %s, want token123:abc", config.Telegram.ApiToken)
	}
	if len(config.Telegram.CriticalReceivers) != 2 {
		t.Fatalf("got %d receivers, want 2", len(config.Telegram.CriticalReceivers))
	}
	if config.Telegram.CriticalReceivers[0].ChatId != "111" {
		t.Errorf("got %s, want 111", config.Telegram.CriticalReceivers[0].ChatId)
	}
	if config.Telegram.CriticalReceivers[1].ChatId != "222" {
		t.Errorf("got %s, want 222", config.Telegram.CriticalReceivers[1].ChatId)
	}

	// Email
	if !config.Email.Enabled {
		t.Error("expected email enabled")
	}
	if config.Email.SmtpSmarthost != "smtp.gmail.com:587" {
		t.Errorf("got %s, want smtp.gmail.com:587", config.Email.SmtpSmarthost)
	}
	if config.Email.SmtpFrom != "test@gmail.com" {
		t.Errorf("got %s, want test@gmail.com", config.Email.SmtpFrom)
	}
	if len(config.Email.AlertReceivers) != 2 {
		t.Fatalf("got %d receivers, want 2", len(config.Email.AlertReceivers))
	}
}

func TestInstallMonitoringRequestJSON(t *testing.T) {
	t.Run("unmarshal full request with chatId lowercase", func(t *testing.T) {
		input := `{
			"grafanaPassword": "pass123",
			"loggingEnabled": true,
			"alertManager": {
				"telegram": {
					"enabled": true,
					"apiToken": "123:abc",
					"criticalReceivers": [{"chatId": "999"}]
				},
				"email": {
					"enabled": false
				}
			}
		}`
		var req InstallMonitoringRequest
		if err := json.Unmarshal([]byte(input), &req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.GrafanaPassword != "pass123" {
			t.Errorf("got %s, want pass123", req.GrafanaPassword)
		}
		if len(req.AlertManager.Telegram.CriticalReceivers) != 1 {
			t.Fatalf("got %d receivers, want 1", len(req.AlertManager.Telegram.CriticalReceivers))
		}
		if req.AlertManager.Telegram.CriticalReceivers[0].ChatId != "999" {
			t.Errorf("got %s, want 999", req.AlertManager.Telegram.CriticalReceivers[0].ChatId)
		}
	})
}
