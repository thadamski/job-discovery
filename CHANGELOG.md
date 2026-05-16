# Changelog

## [0.1.3](https://github.com/thadamski/job-discovery/compare/v0.1.2...v0.1.3) (2026-05-16)


### Bug Fixes

* **ci:** guard fromJson against empty release-please pr output ([812f4f1](https://github.com/thadamski/job-discovery/commit/812f4f173981c9d79da687da53e41a405c3b5f6f))

## [0.1.2](https://github.com/thadamski/job-discovery/compare/v0.1.1...v0.1.2) (2026-05-15)


### Bug Fixes

* **security:** scope job-hunt ingress to hermes-personal only ([c92bf38](https://github.com/thadamski/job-discovery/commit/c92bf3835943f9b0463f657f802dc79ce7f18d4e))

## [0.1.1](https://github.com/thadamski/job-discovery/compare/v0.1.0...v0.1.1) (2026-05-14)


### Bug Fixes

* **deploy:** add GHCR pull secret and imagePullSecrets to cassette overlay ([b2cdd53](https://github.com/thadamski/job-discovery/commit/b2cdd53abec80de48680985c6d5eee6c342b6a03))
* **deploy:** add ingress NetworkPolicy for CNPG pods on port 5432 ([24c1d23](https://github.com/thadamski/job-discovery/commit/24c1d23391dc258faf39b5d775cb5712582514c1))
* **deploy:** allow CloudNativePG pod egress to Kubernetes API server ([41e1758](https://github.com/thadamski/job-discovery/commit/41e17588c3b70eddbe371eb863574ee50dbbf091))
* **deploy:** allow CNPG egress to API server post-DNAT address and use namespace selector for DNS ([54184fa](https://github.com/thadamski/job-discovery/commit/54184fa32bec539b71c51add8d11abcbbc5c7d1a))
* **deploy:** correct CNPG NetworkPolicy pod selector label ([2a9d567](https://github.com/thadamski/job-discovery/commit/2a9d567bdd47341cc990a40afb787d1715ea7434))
* **deploy:** drop ServiceMonitor from base — no Prometheus Operator on cassette ([d488b00](https://github.com/thadamski/job-discovery/commit/d488b00df0eccb9fbaf3b20c5db14d08df67b158))
* **deploy:** replace GHCR pull secret with working token from claude-code-work ([19939a1](https://github.com/thadamski/job-discovery/commit/19939a17ddee100ccdd2f8d01a7a8946bc97bea3))
* tags nil, missing http metrics, page/page_size pagination, migration init container ([3f468b8](https://github.com/thadamski/job-discovery/commit/3f468b8af7e11e38ef63403e7903be4d98101957))
