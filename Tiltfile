# use the existing kind cluster

k8s_context('kind-paw-suite')

# build the admin -service image
docker_build(
    'admin-servive:dev',
    '.',
    dockerfile='services/admin-service/Dockerfile'
)

# apply kubernetes manifests
k8s_yaml(['infra/k8s/mongo/deployment.yaml','infra/k8s/mongo/service.yaml'])
k8s_yaml(['infra/k8s/admin-service/deployment.yaml','infra/k8s/admin-service/service.yaml'])
k8s_yaml(['infra/k8s/nginx/configmap.yaml','infra/k8s/nginx/deployment.yaml','infra/k8s/nginx/service.yaml'])


# tell tilt which kubernetes resource to watch
k8s_resource('admin-service')
k8s_resource('mongo')
k8s_resource(
    'nginx-gateway',
    port_forwards=8080
)