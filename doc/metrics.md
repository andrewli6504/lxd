# Instance metrics
LXD provides metrics for all running instances. Those covers CPU, memory, network, disk and process usage and are meant to be consumed by Prometheus and likely graphed in Grafana.
In cluster environments, LXD will only return the values for instances running on the server being accessed. It's expected that each cluster member will be scraped separately.
The instance metrics are updated when calling the `/1.0/metrics` endpoint.
They are cached for 15s to handle multiple scrapers. Fetching metrics is a relatively expensive operation for LXD to perform so we would recommend scraping at a 30s or 60s rate to limit impact.

## Create metrics certificate
The `/1.0/metrics` endpoint is a special one as it also accepts a `metrics` type certificate.
This kind of certificate is meant for metrics only, and won't work for interaction with instances or any other LXD objects.

Here's how to create a new certificate (this is not specific to metrics):

```bash
openssl req -x509 -newkey rsa:2048 -keyout ~/.config/lxc/metrics.key -nodes -out ~/.config/lxc/metrics.crt -subj "/CN=lxd.local"
```

Now, this certificate needs to be added to the list of trusted clients:

```bash
lxc config trust add ~/.config/lxc/metrics.crt --type=metrics
```

## Add target to Prometheus
In order for Prometheus to scrape from LXD, it has to be added to the targets.

First, one needs to ensure that `core.https_address` is set so LXD can be reached over the network.
This can be done by running:

```bash
lxc config set core.https_address ":8443"
```

Alternatively, one can use `core.metrics_address` which is intended for metrics only.

Second, the newly created certificate and key, as well as the LXD server certificate need to be accessible to Prometheus.
For this, these three files can be copied to `/etc/prometheus/tls`:

```bash
# Create new tls directory
mkdir /etc/prometheus/tls

# Copy newly created certificate and key to tls directory
cp ~/.config/lxc/metrics.crt ~/.config/lxc/metrics.key /etc/prometheus/tls

# Copy LXD server certificate to tls directory
cp /var/snap/lxd/common/lxd/server.crt /etc/prometheus/tls

# Make sure Prometheus can read these files (usually, Prometheus is run as user "prometheus")
chown -R prometheus:prometheus /etc/prometheus/tls
```

Lastly, LXD has to be added as target.
For this, `/etc/prometheus/prometheus.yaml` needs to be edited.
Here's what the config needs to look like:

```yaml
scrape_configs:
  - job_name: lxd
    tls_config:
      ca_file: 'tls/lxd.crt'
      key_file: 'tls/metrics.key'
      cert_file: 'tls/metrics.crt'
    static_configs:
      - targets: ['127.0.0.1:8443']
    metrics_path: '/1.0/metrics'
    scheme: 'https'
```
