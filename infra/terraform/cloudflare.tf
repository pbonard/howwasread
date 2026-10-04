locals {
  cloudflare_account_id = "974f94f87ea3d25fca82e9fb1f408b83"
  # cloudflared routes every hostname of the zone to the envoy gateway, the gateway picks the service by hostname
  envoy_gateway = "http://static-envoy-ingress.envoy-gateway-system.svc.cluster.local:80"
}

resource "cloudflare_zone" "main" {
  account = {
    id = local.cloudflare_account_id
  }
  name = "pbonard.com"
}

resource "cloudflare_dns_record" "apex" {
  zone_id = cloudflare_zone.main.id
  name    = "pbonard.com"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.kind_cluster.id}.cfargotunnel.com"
  type    = "CNAME"
  proxied = true
  # 1 is automatic
  ttl = 1
}

# every subdomain (*.pbonard.com) goes through the tunnel too
resource "cloudflare_dns_record" "wildcard" {
  zone_id = cloudflare_zone.main.id
  name    = "*.pbonard.com"
  content = "${cloudflare_zero_trust_tunnel_cloudflared.kind_cluster.id}.cfargotunnel.com"
  type    = "CNAME"
  proxied = true
  ttl     = 1
}

resource "cloudflare_dns_record" "turn" {
  zone_id = cloudflare_zone.main.id
  name    = "turn.pbonard.com"
  # placeholder, the turn service writes the home ip whenever it changes
  content = "127.0.0.1"
  type    = "A"
  proxied = false
  ttl     = 1

  lifecycle {
    ignore_changes = [content]
  }
}

resource "random_id" "tunnel_secret" {
  byte_length = 35
}

resource "cloudflare_zero_trust_tunnel_cloudflared" "kind_cluster" {
  account_id = local.cloudflare_account_id
  name       = "local-kind-cluster"
  # the ingress rules are kept at cloudflare (the config resource below), not in a local config file
  config_src    = "cloudflare"
  tunnel_secret = random_id.tunnel_secret.b64_std

  lifecycle {
    # the api never returns the secret, an imported tunnel would otherwise be replaced to set it again
    ignore_changes = [tunnel_secret]
  }
}

resource "cloudflare_zero_trust_tunnel_cloudflared_config" "kind_cluster_config" {
  account_id = local.cloudflare_account_id
  tunnel_id  = cloudflare_zero_trust_tunnel_cloudflared.kind_cluster.id

  config = {
    ingress = [
      {
        hostname = "pbonard.com"
        service  = local.envoy_gateway
      },
      {
        hostname = "*.pbonard.com"
        service  = local.envoy_gateway
      },
      {
        service = "http_status:404"
      },
    ]
  }
}

data "cloudflare_zero_trust_tunnel_cloudflared_token" "kind_cluster" {
  account_id = local.cloudflare_account_id
  tunnel_id  = cloudflare_zero_trust_tunnel_cloudflared.kind_cluster.id
}

output "cloudflare_nameservers" {
  value       = cloudflare_zone.main.name_servers
  description = "The nameservers of the zone, already set since the domain is registered at cloudflare"
}

output "cloudflare_zone_id" {
  value       = cloudflare_zone.main.id
  description = "The Cloudflare Zone ID for pbonard.com"
}

output "cloudflare_turn_record_id" {
  value       = cloudflare_dns_record.turn.id
  description = "The DNS Record ID for turn.pbonard.com"
}

output "tunnel_token" {
  value       = data.cloudflare_zero_trust_tunnel_cloudflared_token.kind_cluster.token
  sensitive   = true
  description = "The token required for the cloudflared pod."
}
