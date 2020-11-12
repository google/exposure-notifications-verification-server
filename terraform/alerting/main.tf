module "verification-server" {
  source = "./verification-server"
  monitoring-host-project = (
  var.monitoring-host-project != "" ? var.monitoring-host-project : var.verification-server-project)
  verification-server-project = var.verification-server-project

  apiserver_hosts = var.apiserver_hosts
  adminapi_hosts  = var.adminapi_hosts
  server_hosts    = var.server_hosts

  notification-email = var.notification-email

  https-forwarding-rule = var.https-forwarding-rule
}
