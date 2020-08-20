operator-sdk init --domain=atarazana.com --repo=github.com/atarazana/gramola-operator

operator-sdk create api --group gramola --version v1 --kind AppService --resource=true --controller=true