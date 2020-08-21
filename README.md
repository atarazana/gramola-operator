operator-sdk init --domain=atarazana.com --repo=github.com/atarazana/gramola-operator

operator-sdk create api --group gramola --version v1 --kind AppService --resource=true --controller=true


 k port-forward gramola-operator-catalog-dt7d6 50051

 grpcurl -plaintext -d '{"pkgName":"gramola-operator","channelName":"alpha"}' localhost:50051 api.Registry/GetBundleForChannel


git clone https://github.com/operator-framework/operator-lifecycle-manager
cd operator-lifecycle-manager
make run-console-local