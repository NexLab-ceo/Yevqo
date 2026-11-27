-- Preserve whether Yevqo owns the Tunnel ingress or only the DNS record.
-- Existing route rows were created for remotely-managed tunnels, so default to true.
ALTER TABLE site_cloudflare_routes
    ADD COLUMN ingress_managed TINYINT(1) NOT NULL DEFAULT 1;
