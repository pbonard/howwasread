resource "helm_release" "envoy_gateway" {
  name             = "eg"
  repository       = "oci://docker.io/envoyproxy"
  chart            = "gateway-helm"
  namespace        = "envoy-gateway-system"
  create_namespace = true

  # depends_on = [
  #   module.cluster1.eks_managed_node_groups
  # ]
}

resource "helm_release" "strimzi" {
  name             = "my-strimzi-cluster-operator"
  repository       = "oci://quay.io/strimzi-helm"
  chart            = "strimzi-kafka-operator"
  namespace        = "kafka-system"
  create_namespace = true

  # depends_on = [
  #   module.cluster1.eks_managed_node_groups
  # ]
}

resource "helm_release" "k8ssandra" {
  name             = "k8ssandra"
  repository       = "https://helm.k8ssandra.io/stable"
  chart            = "k8ssandra-operator"
  namespace        = "k8ssandra"
  create_namespace = true


  depends_on = [helm_release.cert_manager]

  # set {
  #   name  = "cassandra.version"
  #   value = "4.0.0"
  # }
}

resource "helm_release" "flink_operator" {
  name             = "flink-operator"
  repository       = "https://downloads.apache.org/flink/flink-kubernetes-operator-1.15.0/"
  chart            = "flink-kubernetes-operator"
  namespace        = "flink"
  create_namespace = true

  depends_on = [helm_release.cert_manager]

  set {
    name  = "webhook.create"
    value = "true"
  }
}

resource "helm_release" "trust_manager" {
  name       = "trust-manager"
  repository = "https://charts.jetstack.io"
  chart      = "trust-manager"
  namespace  = "cert-manager"

  set {
    name  = "app.trust.namespace"
    value = "cert-manager"
  }
}

resource "helm_release" "cert_manager" {
  name             = "cert-manager"
  repository       = "https://charts.jetstack.io"
  chart            = "cert-manager"
  namespace        = "cert-manager"
  create_namespace = true

  set {
    name  = "installCRDs"
    value = "true"
  }

  set {
    name  = "config.apiVersion"
    value = "controller.config.cert-manager.io/v1alpha1"
  }

  set {
    name  = "config.kind"
    value = "ControllerConfiguration"
  }

  set {
    name  = "config.enableGatewayAPI"
    value = "true"
  }

  # values = [
  #   yamlencode({
  #     nodeSelector = {
  #       "node.kubernetes.io/instance-type" = local.idle_node
  #     }
  #     webhook = {
  #       nodeSelector = {
  #         "node.kubernetes.io/instance-type" = local.idle_node
  #       }
  #     }
  #     cainjector = {
  #       nodeSelector = {
  #         "node.kubernetes.io/instance-type" = local.idle_node
  #       }
  #     }
  #   })
  # ]

  # depends_on = [
  #   module.cluster1.eks_managed_node_groups
  # ]
}

variable "argocd_github_webhook_secret" {
  description = "Secret of the github webhook, argo cd rejects webhook calls not signed with it"
  type        = string
  sensitive   = true
}

resource "helm_release" "argocd" {
  name       = "argocd"
  repository = "https://argoproj.github.io/argo-helm"
  chart      = "argo-cd"
  # pinned, so terraform apply never upgrades argo cd by surprise (chart 10.9.6 = argo cd v3.5.3)
  version          = "10.9.6"
  namespace        = "argocd"
  create_namespace = true

  values = [
    yamlencode({
      server = {
        # plain http, public tls ends at cloudflare; the ui is only reached through kubectl port-forward,
        # the gateway exposes nothing but POST /api/webhook (infra/k8s/bootstrap/argocd-webhook.yaml)
        extraArgs = ["--insecure"]
        service = {
          type = "ClusterIP"
        }
        replicas    = 1
        autoscaling = { enabled = false }
      }
      repoServer = {
        replicas    = 1
        autoscaling = { enabled = false }
      }
      controller = {
        replicas = 1
      }
      applicationSet = {
        replicaCount = 1
      }
      redis-ha = {
        enabled = false
      }
      notifications = {
        enabled = false
      }
      # no sso, login is the admin account
      dex = {
        enabled = false
      }
      configs = {
        cm = {
          # resource types the holiday app never deploys, the controller stops caching and watching them.
          # appended to the chart's default exclusions. Remove a group here before the chart starts using it
          resourceExclusionsAdditional = [
            # kafka (strimzi)
            { apiGroups = ["kafka.strimzi.io", "core.strimzi.io"], kinds = ["*"] },
            # vitess
            { apiGroups = ["planetscale.com"], kinds = ["*"] },
            # cassandra (k8ssandra)
            {
              apiGroups = [
                "k8ssandra.io", "cassandra.datastax.com", "config.k8ssandra.io", "control.k8ssandra.io",
                "medusa.k8ssandra.io", "reaper.k8ssandra.io", "replication.k8ssandra.io", "stargate.k8ssandra.io",
              ]
              kinds = ["*"]
            },
            # certificates
            { apiGroups = ["cert-manager.io", "acme.cert-manager.io", "trust.cert-manager.io"], kinds = ["*"] },
            # valkey, grafana alloy, k3s internals, experimental gateway api
            # (sealed secrets stay watched, so they can be managed by argo cd later)
            {
              apiGroups = [
                "valkey.io", "monitoring.grafana.com", "helm.cattle.io", "k3s.cattle.io",
                "gateway.networking.x-k8s.io",
              ]
              kinds = ["*"]
            },
            # the noisiest type, health never reads it
            { apiGroups = ["", "events.k8s.io"], kinds = ["Event"] },
          ]
        }
      }
    })
  ]

  # kept out of the values and the repo, set with TF_VAR_argocd_github_webhook_secret
  set_sensitive {
    name  = "configs.secret.githubSecret"
    value = var.argocd_github_webhook_secret
  }
}

# flux keeps the cluster at what infra/k8s/helm on main says, see infra/k8s/bootstrap/flux.yaml.
# install by hand with -target; only the controllers this setup uses, ci writes the image tags itself
# resource "helm_release" "flux" {
#   name       = "flux"
#   repository = "https://fluxcd-community.github.io/helm-charts"
#   chart      = "flux2"
#   # pinned, so terraform apply never upgrades flux by surprise (chart 2.19.1 = flux 2.9.5)
#   version          = "2.19.1"
#   namespace        = "flux-system"
#   create_namespace = true
#
#   values = [
#     yamlencode({
#       # fetches the repo
#       sourceController = { create = true }
#       # applies flux.yaml's objects when they live in git, kept for later
#       kustomizeController = { create = true }
#       # runs helm upgrade for the HelmRelease
#       helmController = { create = true }
#       # receives the github webhook
#       notificationController = { create = true }
#       # image tag automation, ci already writes the tags
#       imageAutomationController = { create = false }
#       imageReflectionController = { create = false }
#     })
#   ]
# }

resource "helm_release" "sealed_secrets" {
  name       = "sealed-secrets-controller"
  repository = "oci://registry-1.docker.io/bitnamicharts"
  chart      = "sealed-secrets"
  namespace  = "kube-system"

  # values = [
  #   yamlencode({
  #     nodeSelector = {
  #       "node.kubernetes.io/instance-type" = local.idle_node
  #     }
  #   })
  # ]

  # depends_on = [
  #   module.cluster1.eks_managed_node_groups
  # ]
}

resource "helm_release" "reflector" {
  name       = "reflector-controller"
  repository = "oci://ghcr.io/emberstack/helm-charts"
  chart      = "reflector"
  namespace  = "kube-system"

  # values = [
  #   yamlencode({
  #     nodeSelector = {
  #       "node.kubernetes.io/instance-type" = local.idle_node
  #     }
  #   })
  # ]

  # depends_on = [
  #   module.cluster1.eks_managed_node_groups
  # ]
}

resource "helm_release" "alloy" {
  name             = "alloy"
  repository       = "https://grafana.github.io/helm-charts"
  chart            = "alloy"
  namespace        = "monitoring"
  create_namespace = true

  # Passing the complete Alloy River configuration via YAML values
  values = [
    <<-EOF
    alloy:
      envFrom:
        - secretRef:
            name: grafanacloud

      configMap:
        create: true
        content: |
          // 2. Read the environment variable in the Loki config
          loki.write "grafanacloud" {
            endpoint {
              url = "https://logs-prod-030.grafana.net/loki/api/v1/push"

              basic_auth {
                username = "1654261"
                password = sys.env("GRAFANA_CLOUD_TOKEN")
              }
            }
          }

          // ==========================================
          // 2. KUBERNETES DISCOVERY & BASE LABELS
          // ==========================================
          discovery.kubernetes "pods" {
            role = "pod"
          }

          discovery.relabel "base_labels" {
            targets = discovery.kubernetes.pods.targets

            rule {
              source_labels = ["__meta_kubernetes_namespace"]
              target_label  = "namespace"
            }
            rule {
              source_labels = ["__meta_kubernetes_pod_name"]
              target_label  = "pod"
            }
            rule {
              source_labels = ["__meta_kubernetes_pod_container_name"]
              target_label  = "container"
            }
          }

          // ==========================================
          // 3. Default
          // ==========================================
          discovery.relabel "none" {
            targets = discovery.relabel.base_labels.output
            rule {
              action        = "keep"
              // Targets the new label we added to your Helm template
              source_labels = ["__meta_kubernetes_pod_label_framework"]
              regex         = "none"
            }
          }

          loki.source.kubernetes "default_logs" {
            targets    = discovery.relabel.none.output
            forward_to = [loki.write.grafanacloud.receiver]
          }

          // ==========================================
          // 4. SPRING BOOT APPLICATIONS
          // ==========================================
          discovery.relabel "spring_apps" {
            targets = discovery.relabel.base_labels.output
            rule {
              action        = "keep"
              source_labels = ["__meta_kubernetes_pod_label_framework"]
              regex         = "spring-boot"
            }
          }

          loki.source.kubernetes "spring_logs" {
            targets    = discovery.relabel.spring_apps.output
            // Route to the multiline processor first
            forward_to = [loki.process.spring_multiline.receiver]
          }

          loki.process "spring_multiline" {
            // Forward processed logs to Loki
            forward_to = [loki.write.grafanacloud.receiver]

            stage.multiline {
              // Matches standard Spring Boot timestamp format: YYYY-MM-DD HH:MM:SS.mmm
              firstline     = "^\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}\\.\\d{3}"
              max_wait_time = "3s"
            }
          }
    EOF
  ]
}

resource "helm_release" "valkey_operator" {
  name             = "valkey-operator"
  repository       = "https://valkey.io/valkey-helm"
  chart            = "valkey-operator"
  namespace        = "valkey-system"
  create_namespace = true

  # values = [
  #   yamlencode({
  #     nodeSelector = {
  #       "node.kubernetes.io/instance-type" = local.idle_node
  #     }
  #   })
  # ]

  # depends_on = [
  #   module.cluster0.eks_managed_node_groups
  # ]
}

# resource "helm_release" "metrics_server" {
#   name       = "metrics-server"
#   repository = "https://kubernetes-sigs.github.io/metrics-server/"
#   chart      = "metrics-server"
#   namespace  = "kube-system"
#
#   depends_on = [
#     module.cluster1.eks_managed_node_groups
#   ]
# }
