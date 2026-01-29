# use the existing kind cluster
k8s_context('kind-paw-suite')

# build the admin -service image
docker_build(
    'admin-service:dev',
    '.',
    dockerfile='services/admin-service/Dockerfile'
)

# build the data-service image
docker_build(
    'data-service:dev',
    '.',
    dockerfile='services/data-service/Dockerfile'
)

# apply kubernetes manifests
k8s_yaml(['infra/k8s/mongo/deployment.yaml','infra/k8s/mongo/service.yaml'])
k8s_yaml(['infra/k8s/admin-service/deployment.yaml','infra/k8s/admin-service/service.yaml'])
k8s_yaml(['infra/k8s/nginx/configmap.yaml','infra/k8s/nginx/deployment.yaml','infra/k8s/nginx/service.yaml'])
k8s_yaml(['infra/k8s/data-service/deployment.yaml','infra/k8s/data-service/service.yaml'])
k8s_yaml(['infra/k8s/rabbitmq/deployment.yaml', 'infra/k8s/rabbitmq/service.yaml'])

# tell tilt which kubernetes resource to watch
# Add resource dependencies to ensure RabbitMQ starts first
k8s_resource('rabbitmq',
    port_forwards=['15672:15672', '5672:5672'],
    labels=['infra']
)

k8s_resource('mongo', labels=['infra'])

k8s_resource('admin-service',
    resource_deps=['rabbitmq', 'mongo'],
    labels=['backend-service']
)

k8s_resource('data-service',
    resource_deps=['rabbitmq', 'mongo'],
    labels=['backend-service']
)

k8s_resource(
    'nginx-gateway',
    port_forwards=8080,
    labels=['infra']
)