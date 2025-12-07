# Version Compatibility Matrix

This document tracks tested combinations of framework versions in Forge-generated workspaces.

## Current Recommended Versions (December 2025)

| Tool        | Version | Notes                                   |
| ----------- | ------- | --------------------------------------- |
| **Angular** | 21.0.2  | Latest stable, full Bazel support       |
| **Go**      | 1.23.4  | Latest stable in 1.23.x series          |
| **NestJS**  | 11.1.9  | Latest stable, Node 24.x compatible     |
| **Node.js** | 24.11.1 | Current LTS release                     |
| **Bazel**   | 7.4.1   | Latest stable, required for Angular 21+ |

## Tested Combinations

### Current (Recommended)

✅ **Fully Tested & Supported**

- Angular 21.0.3 + Bazel 7.4.1 + Node 24.11.1
- Go 1.23.4 + Bazel 7.4.1
- NestJS 11.1.9 + Node 24.11.1 + Bazel 7.4.1

### Previous Stable

⚠️ **Supported but not recommended for new projects**

- Angular 20.x + Bazel 7.3.x + Node 22.x
- Go 1.22.x + Bazel 7.3.x
- NestJS 10.x + Node 20.x + Bazel 7.3.x

## Bazel Rule Versions

These versions are managed in `MODULE.bazel.tmpl` and tested as a cohesive unit:

| Rule                 | Version | Purpose                     |
| -------------------- | ------- | --------------------------- |
| rules_go             | 0.50.1  | Go build rules              |
| gazelle              | 0.39.1  | BUILD file generator        |
| rules_nodejs         | 6.3.2   | Node.js toolchain           |
| aspect_rules_js      | 2.8.2   | JavaScript/TypeScript rules |
| aspect_rules_ts      | 3.7.1   | TypeScript compilation      |
| aspect_rules_esbuild | 0.24.0  | JS bundling                 |
| rules_oci            | 2.0.0   | Container images            |

## Known Incompatibilities

### Angular

- ❌ **Angular 21+ requires Bazel 7.4+** - Earlier Bazel versions lack required features
- ❌ **Angular 21+ requires Node 20+** - Earlier Node versions unsupported
- ⚠️ **Angular 19/20 with Bazel 7.4+** - Works but deprecated, update to Angular 21

### Go

- ❌ **Go 1.25+ with rules_go < 0.50** - May have module resolution issues
- ⚠️ **Go 1.22.x** - Security updates only, migrate to 1.23.x

### NestJS

- ❌ **NestJS 11+ requires Node 20+** - Earlier versions incompatible
- ⚠️ **NestJS 10.x** - Maintenance mode, update to 11.x recommended

## Update Strategy

### When to Update

- **Security patches**: Update immediately (check Dependabot PRs)
- **Minor versions**: Update quarterly or when needed features available
- **Major versions**: Review breaking changes, test thoroughly, coordinate team

### How to Update

1. **Review Dependabot PRs**: Dependabot monitors `package.json` and `go.mod` files
2. **Test locally**: Merge PR and run `forge build` to verify compatibility
3. **Update forge.json**: Manually sync versions to `toolVersions` section
4. **Update this document**: Add new tested combinations

### Rollback Procedure

If an update breaks builds:

1. Revert the PR that updated dependencies
2. Restore previous versions in `forge.json` `toolVersions`
3. Run `forge clean --cache` to clear stale build artifacts
4. Rebuild with `forge build`

## Version Validation

Forge validates versions during build:

```bash
forge build
```

Will warn if `toolVersions` are >6 months old or incompatible with latest known stable releases.

## Contributing

When testing new version combinations:

1. Create test workspace: `forge new test-versions`
2. Update `forge.json` with new versions
3. Run full build: `forge build`
4. Test generated services and frontends
5. Document results in PR to this file

## Resources

- [Angular Compatibility Guide](https://angular.io/guide/update)
- [Go Release History](https://go.dev/doc/devel/release)
- [NestJS Migration Guides](https://docs.nestjs.com/migration-guide)
- [Bazel Release Notes](https://github.com/bazelbuild/bazel/releases)
- [Node.js Release Schedule](https://nodejs.org/en/about/releases/)

## Questions?

For version compatibility questions or issues, open an issue on [forge-cli repository](https://github.com/dosanma1/forge-cli/issues).
