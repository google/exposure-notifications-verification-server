<!-- TOC depthFrom:2 -->

- [Account setup](#account-setup)
- [Inviting new admins](#inviting-new-admins)
- [Creating new realms](#creating-new-realms)
- [View realm information](#view-realm-information)
- [Joining realms](#joining-realms)
- [Create system SMS configuration](#create-system-sms-configuration)
- [Create system SMTP configuration](#create-system-smtp-configuration)
- [Configure ENX redirect service](#configure-enx-redirect-service)
- [Adding ENX redirect domains](#adding-enx-redirect-domains)
- [Clearing caches](#clearing-caches)
- [Getting system information](#getting-system-information)

<!-- /TOC -->

# System admin guide

This guide describes common system administration tasks. Note this is different
from infrastructure administration. System administrators are super users of the
system and are responsible for creating realms and configuring system-level
parameters that apply to all realms.

## Account setup

To become a system admin, an existing system admin must invite you to join. You
will receive an email notification. If you are already a member of the system
(as a non-system admin), you will be promoted. If you are not already a member
of the system, an account will be provisioned automatically and you will be
prompted to reset your password.

Newly provisioned databases will be bootstrapped with
[super@example.com](/docs/production.md#user-administration) from which you may
create your first real accounts.

## Inviting new admins

As a system admin, you can invite new system admins. Before inviting a system
admin, perform due diligence to ensure the user requires admin permissions.
System-level administration is very a privileged role and should only be
delegated to trusted users.

To invite a new system admin, visit the `/admin/users` URL. You can access it
directly via:

```text
https://<your-domain>/admin/users
```

Or by choosing "System admin" from the dropdown and selecting the "Users" tab.

![System Admin list](images/system-admin/users-index.png "System Admin list")

Click the "+" to launch the New System Admin page.

![New System Admin](images/system-admin/new-system-admin.png "New System Admin")

Click "Create System Admin" to create the user. If the user already exists in
the system, they will be promoted. Otherwise, an account will be automatically
provisioned and they will receive an email with password reset instructions.

## Creating new realms

To create a new realm for a health authority, visit the `/admin/realms` URL. You
can access it directly via:

```text
https://<your-domain>/admin/realms
```

Or by choosing "System admin" from the dropdown and selecting the "Realms" tab.

![Realms list](images/system-admin/realms-index.png "Realms list")

Click the "+" to launch the New Realm page. **Realms cannot be deleted!**

![New Realm](images/system-admin/new-realm.png "New Realm")

The realm should provide you the values for the `iss` and `aud` values, and
these values must match what is supplied to the key server.

Note that must realm properties are immutable after creation!

## View realm information

As a system administrator, you can view high-level realm information such as
abuse prevention statistics, recent events, and mobile apps. To view high-level
realm information, visit the `/admin/realms` URL. You can access it directly
via:

```text
https://<your-domain>/admin/realms
```

Or by choosing "System admin" from the dropdown and selecting the "Realms" tab.

![Realms list](images/system-admin/realms-index.png "Realms list")

Click on the realm name in the realms list view:

![Realms show](images/system-admin/realm-show.png "Realm show")

You can see abuse prevention statistics and deep links to events and mobile apps
for this realm. You do **not** need to join the realm to see this data.

## Joining realms

As a system administrator, you can join a realm as a realm administrator to help
debug issues like misconfigurations. To join a realm as an administrator, visit
the `/admin/realms` URL. You can access it directly via:

```text
https://<your-domain>/admin/realms
```

Or by choosing "System admin" from the dropdown and selecting the "Realms" tab.

![Realms list](images/system-admin/realms-index.png "Realms list")

Click on the realm name in the realms list view:

![Realms show](images/system-admin/realm-show.png "Realm show")

Scroll to the bottom and click "Join realm". **This event is audited and
logged!**

## Create system SMS configuration

The system can optionally provide a system-level SMS configuration and then
share that configuration on a per-realm basis. Even if a system-level SMS
configuration is defined, individual realms can still provide their own
configurations. To provide a system-level SMS configuration, visit the
`/admin/sms` URL. You can access it directly via:

```text
https://<your-domain>/admin/sms
```

Or by choosing "System admin" from the dropdown and selecting the "SMS" tab.

![SMS show](images/system-admin/sms-show.png "SMS show")

Complete the information for the [Twilio](https://twilio.com) credentials and
click the "Update system SMS config" button.

Upon saving the system SMS configuration, the "New realm" and "Realm show" pages
will have a new optional setting to share this system SMS configuration with
that realm.

![Realm show SMS settings](images/system-admin/realm-show-sms.png "Realm show SMS settings")

## Create system SMTP configuration

The system can optionally provide a system-level email configuration and then
share that configuration on a per-realm basis. Even if a system-level email
configuration is defined, individual realms can still provide their own
configurations. To provide a system-level email configuration, visit the
`/admin/email` URL. You can access it directly via:

```text
https://<your-domain>/admin/email
```

Or by choosing "System admin" from the dropdown and selecting the "Email" tab.

![Email show](images/system-admin/email-show.png "Email show")

Complete the information for the SMTP credentials.

Upon saving the system email configuration, the "New realm" and "Realm show"
pages will have a new optional setting to share this system email configuration
with that realm.

![Realm show email settings](images/system-admin/realm-show-email.png "Realm show email settings")

## Configure ENX redirect service

The system can optionally host an ENX redirector service. This service provides
realm-specific deep-links on publicly-accessible URLs to help drive app adoption
and code redemption. **To configure the ENX redirect service, you must acquire a
unique domain solely for the purposes of this service.**

1.  Compile a list of all possible realms and regions which may want the
    redirector service. Since **adding new domains can cause downtime** (due to
    certificate provisioning), it is recommended that you are liberal in your
    list. You can add entries for realms which do not exist or realms that are
    not using ENX today.

1.  Update and apply your Terraform configuration with the following new values:

    -   `enx_redirect_domain` - root TLD for enx-redirect service links (e.g.
        mydomain.com).

    -   `enx_redirect_domain_map` - mapping of regions to hosts (e.g.
        `{host="us-ca.<domain>", region="us-ca"}, ...`)

## Adding ENX redirect domains

If you just add to the ENX redirect domain map, it will cause an SSL outage. To
solve this, we need to create the new certificate, deploy it, wait, and then
delete the old one.

This approach doesn't allow for removing domains, but that doesn't need to be
done.

1. In the `main.tf` for the project to modify.

   * Add new domains to the `enx_redirect_domain_map_add` section

   * Increment / set `redirect_cert_generation_next` to be 1 higher than
     the value in `redirect_cert_generation`

2. Apply this change.

3. Verify that the new certificate is created and installed the load balancer

    * [Cloud Console Certificates Page](https://console.cloud.google.com/loadbalancing/advanced/sslCertificates/list)

    * Use `curl -V` to verify the new domain name has a certificate in
      the load balancer.

    * __This could take up to an hour__

4. Promote the new certificate to be the only one.

 This is done by backing up the existing terraform state and doing a manual
 delete and move of the certificate status.

```shell
terraform state pull > somefile.tfstate
terraform state rm module.en.google_compute_managed_ssl_certificate.enx-redirect
terraform state mv module.en.google_compute_managed_ssl_certificate.enx-redirect-next module.en.google_compute_managed_ssl_certificate.enx-redirect
```

5. Now, combine the states.

   * Move the contents of `enx_redirect_domain_map_add` to
     `enx_redirect_domain_map`. The moved entries must be at the END of the
     list, otherwise a diff will occur on the next step.

   * Set `redirect_cert_generation` to be the same value as
     `redirect_cert_generation_next`. Leave that value in
     `redirect_cert_generation_next` until the next time a redirect domain
     is added.

6. Terraform apply. The only SSL diff should be removing the old
   certificate from the load balancer.

7. Manually delete the old certificate from the cloud console certificates page.

## Clearing caches

In some situations, it may be beneficial to clear certain cached data in the
system. For example, if a health authority pushes out a bad configuration, it
may be cached for up to 30 minutes. System administrators can clear various
caches. To clear caches, visit the `/admin/caches` URL. You can access it
directly via:

```text
https://<your-domain>/admin/caches
```

Or by choosing "System admin" from the dropdown and selecting the "Caches" tab.

![Caches index](images/system-admin/caches-index.png "Caches index")

Each cache is self-described under the name. Press the big red button to clear
the cache. You will be prompted to confirm.

## Getting system information

In some situations, the server engineering team may request build information for your system. To access the build information, visit the `/admin/info` URL. You can access it
directly via:

```text
https://<your-domain>/admin/info
```

Or by choosing "System admin" from the dropdown and selecting the "Info" tab.

![Info show](images/system-admin/info-show.png "Info show")

Supply this information when requested.
