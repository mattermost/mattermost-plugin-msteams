# FAQ
- [How is encryption handled at rest and in motion?](https://github.com/darias416/mattermost-plugin-msteams-sync/blob/FAQs/docs/FAQs.md#are-there-any-database-or-network-security-considerations)
- [Are there any database or network security considerations?](## Are there any database or network security considerations?)
- [Are there any compliance considerations (ie. GDPR, PCI)?](## Are there any compliance considerations (ie. GDPR, PCI)?)
- [How often will users Sync from MS Teams to Mattermost?](## How often will users Sync from MS Teams to Mattermost?)
- [Is a service account required for this integration to sync users from MS Teams to Mattermost?](## Is a service account required for this integration to sync users from MS Teams to Mattermost?)

## How is encryption handled at rest and in motion?

Everything is stored in the Mattermost databases. AES encryption is used to encrypt the MS Teams auth/access token. Other encryption at rest would be dependent on how the Mattermost instance is setup. All communication between the plugin and MS Teams/Graph API are conducted over SSL/HTTPS.

## Are there any database or network security considerations?

There is nothing specific to the plugin that is beyond what would apply to the Mattermost instance.

## Are there any compliance considerations (ie. GDPR, PCI)?

There is nothing specific to the plugin that is beyond what would apply to the Mattermost instance.

## How often will users Sync from MS Teams to Mattermost?

The frequency of user syncing is customizable within the plugin configuration page.

## Is a service account required for this integration to sync users from MS Teams to Mattermost?

No, user syncing is done by the "application" itself.

## How is the plugin architectured?

The architecture diagram is below:

![MS Teams Sync Diagram v1.0](brightscout-msteams-sync-v1.0.png)


