
```
# Install CRD
make install

# Deploy controller
make deploy IMG=satosawa/k8s-at:latest

# Create sample at
kubectl create -f config/samples/batch_v1_at.yaml
```
