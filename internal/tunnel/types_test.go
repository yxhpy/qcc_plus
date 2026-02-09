package tunnel

import (
	"testing"
)

func TestTunnelConfig(t *testing.T) {
	tests := []struct {
		name   string
		config TunnelConfig
		want   TunnelConfig
	}{
		{
			name: "完整配置",
			config: TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test-subdomain",
				LocalAddr: "http://localhost:8000",
				Zone:      "example.com",
			},
			want: TunnelConfig{
				APIToken:  "test-token",
				Subdomain: "test-subdomain",
				LocalAddr: "http://localhost:8000",
				Zone:      "example.com",
			},
		},
		{
			name: "最小配置",
			config: TunnelConfig{
				APIToken:  "token",
				Subdomain: "sub",
			},
			want: TunnelConfig{
				APIToken:  "token",
				Subdomain: "sub",
				LocalAddr: "",
				Zone:      "",
			},
		},
		{
			name:   "空配置",
			config: TunnelConfig{},
			want:   TunnelConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.APIToken != tt.want.APIToken {
				t.Errorf("APIToken = %v, want %v", tt.config.APIToken, tt.want.APIToken)
			}
			if tt.config.Subdomain != tt.want.Subdomain {
				t.Errorf("Subdomain = %v, want %v", tt.config.Subdomain, tt.want.Subdomain)
			}
			if tt.config.LocalAddr != tt.want.LocalAddr {
				t.Errorf("LocalAddr = %v, want %v", tt.config.LocalAddr, tt.want.LocalAddr)
			}
			if tt.config.Zone != tt.want.Zone {
				t.Errorf("Zone = %v, want %v", tt.config.Zone, tt.want.Zone)
			}
		})
	}
}

func TestZone(t *testing.T) {
	tests := []struct {
		name string
		zone Zone
		want Zone
	}{
		{
			name: "完整Zone",
			zone: Zone{
				ID:   "zone-123",
				Name: "example.com",
			},
			want: Zone{
				ID:   "zone-123",
				Name: "example.com",
			},
		},
		{
			name: "空Zone",
			zone: Zone{},
			want: Zone{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.zone.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", tt.zone.ID, tt.want.ID)
			}
			if tt.zone.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", tt.zone.Name, tt.want.Name)
			}
		})
	}
}

func TestTunnel(t *testing.T) {
	tests := []struct {
		name   string
		tunnel Tunnel
		want   Tunnel
	}{
		{
			name: "完整Tunnel",
			tunnel: Tunnel{
				ID:     "tunnel-123",
				Name:   "my-tunnel",
				Secret: "secret-key",
			},
			want: Tunnel{
				ID:     "tunnel-123",
				Name:   "my-tunnel",
				Secret: "secret-key",
			},
		},
		{
			name:   "空Tunnel",
			tunnel: Tunnel{},
			want:   Tunnel{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tunnel.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", tt.tunnel.ID, tt.want.ID)
			}
			if tt.tunnel.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", tt.tunnel.Name, tt.want.Name)
			}
			if tt.tunnel.Secret != tt.want.Secret {
				t.Errorf("Secret = %v, want %v", tt.tunnel.Secret, tt.want.Secret)
			}
		})
	}
}

func TestDNSRecord(t *testing.T) {
	tests := []struct {
		name   string
		record DNSRecord
		want   DNSRecord
	}{
		{
			name: "完整DNSRecord",
			record: DNSRecord{
				ID:      "record-123",
				Name:    "test.example.com",
				Content: "tunnel-id.cfargotunnel.com",
				Type:    "CNAME",
			},
			want: DNSRecord{
				ID:      "record-123",
				Name:    "test.example.com",
				Content: "tunnel-id.cfargotunnel.com",
				Type:    "CNAME",
			},
		},
		{
			name:   "空DNSRecord",
			record: DNSRecord{},
			want:   DNSRecord{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.record.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", tt.record.ID, tt.want.ID)
			}
			if tt.record.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", tt.record.Name, tt.want.Name)
			}
			if tt.record.Content != tt.want.Content {
				t.Errorf("Content = %v, want %v", tt.record.Content, tt.want.Content)
			}
			if tt.record.Type != tt.want.Type {
				t.Errorf("Type = %v, want %v", tt.record.Type, tt.want.Type)
			}
		})
	}
}

