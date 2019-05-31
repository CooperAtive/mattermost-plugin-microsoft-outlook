# Outlook Plugin

WebApp portion disabled for now (from plugin.json, removed webapp portion)

Install the plugin using `make deploy` or manual install. Configure the settings at http://localhost:8065/admin_console/integrations/plugin_outlook. You'll need a clientID and secret.

Once you have a mattermost server running (assuming you're on port 8065), login and visit
http://localhost:8065/plugins/outlook/oauth/connect.

This should kick off the process to ask for permission to connect to a Microsoft account.

If it all works you should recieve a DM (from the user specified in settings), containing your email address and name attached to the Microsoft account you verified.
