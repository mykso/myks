# Integration tests

This directory contains configuration for myks' integration tests.

## Test areas

### Inheritance

This examples tests the full power of myks inheritance for helm and ytt rendering leveraging:

- config from the base app
- config from environment group level
- config from application level

Both environment group level and application level are rendering with prototype override config as well as application specific config.

### Static files

This example tests the ability to correctly render static files from the `static` directories.
