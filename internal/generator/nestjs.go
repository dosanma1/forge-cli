package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// NestJSServiceGenerator generates a new NestJS microservice.
type NestJSServiceGenerator struct {
	engine *template.Engine
}

// NewNestJSServiceGenerator creates a new NestJS service generator.
func NewNestJSServiceGenerator() *NestJSServiceGenerator {
	return &NestJSServiceGenerator{
		engine: template.NewEngine(),
	}
}

// Name returns the generator name.
func (g *NestJSServiceGenerator) Name() string {
	return "nestjs-service"
}

// Description returns the generator description.
func (g *NestJSServiceGenerator) Description() string {
	return "Generate a new NestJS microservice"
}

// Generate creates a new NestJS service.
func (g *NestJSServiceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	serviceName := opts.Name
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}

	// Validate name
	if err := workspace.ValidateName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}

	// Get workspace root
	workspaceRoot := opts.OutputDir
	if workspaceRoot == "" {
		var err error
		workspaceRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Determine service path using workspace.paths or default
	servicesPath := "backend/services"
	if config.Workspace.Paths != nil && config.Workspace.Paths.Services != "" {
		servicesPath = config.Workspace.Paths.Services
	}

	serviceDir := filepath.Join(workspaceRoot, servicesPath, serviceName)

	// Check if service already exists
	if _, err := os.Stat(serviceDir); err == nil {
		return fmt.Errorf("service %s already exists at %s", serviceName, serviceDir)
	}

	if opts.DryRun {
		fmt.Printf("Would create NestJS service: %s at %s\n", serviceName, serviceDir)
		return nil
	}

	// Create service directory structure
	dirs := []string{
		serviceDir,
		filepath.Join(serviceDir, "src"),
		filepath.Join(serviceDir, "src", "health"),
		filepath.Join(serviceDir, "test"),
		filepath.Join(serviceDir, "deploy", "helm"),
		filepath.Join(serviceDir, "deploy", "cloudrun"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Determine registry
	registry := "gcr.io/your-project"
	if opts.Data != nil {
		if r, ok := opts.Data["registry"].(string); ok && r != "" {
			registry = r
		}
	}
	if config.Workspace.Docker != nil && config.Workspace.Docker.Registry != "" {
		registry = config.Workspace.Docker.Registry
	}

	// Generate files
	data := map[string]interface{}{
		"ServiceName": serviceName,
		"Registry":    registry,
	}

	files := map[string]string{
		"package.json":                    nestJSPackageJSON,
		"tsconfig.json":                   nestJSTSConfig,
		"nest-cli.json":                   nestJSCLIConfig,
		".eslintrc.js":                    nestJSESLintConfig,
		".prettierrc":                     nestJSPrettierConfig,
		"Dockerfile":                      nestJSDockerfile,
		"README.md":                       nestJSReadme,
		"src/main.ts":                     nestJSMainTS,
		"src/app.module.ts":               nestJSAppModule,
		"src/app.controller.ts":           nestJSAppController,
		"src/app.service.ts":              nestJSAppService,
		"src/health/health.controller.ts": nestJSHealthController,
		"test/app.e2e-spec.ts":            nestJSE2ETest,
		"test/jest-e2e.json":              nestJSJestE2EConfig,
		"deploy/helm/values.yaml":         nestJSHelmValues,
		"deploy/cloudrun/service.yaml":    nestJSCloudRunService,
	}

	for filePath, content := range files {
		fullPath := filepath.Join(serviceDir, filePath)
		rendered, err := g.engine.Render(content, data)
		if err != nil {
			return fmt.Errorf("failed to render template for %s: %w", filePath, err)
		}

		if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}
	}

	// Register service in forge.json
	project := workspace.Project{
		Name: serviceName,
		Type: workspace.ProjectTypeNestJSService,
		Root: filepath.Join(servicesPath, serviceName),
		Tags: []string{"backend", "nestjs", "service"},
		Build: &workspace.ProjectBuildConfig{
			NodeVersion: "22.0.0",
			Registry:    registry,
			Dockerfile:  "Dockerfile",
		},
		Deploy: &workspace.ProjectDeployConfig{
			Targets:    []string{"helm", "cloudrun"},
			ConfigPath: "deploy",
			Helm: &workspace.ProjectDeployHelm{
				Port:       3000,
				HealthPath: "/health",
			},
			CloudRun: &workspace.ProjectDeployCloudRun{
				Port:         3000,
				CPU:          "1",
				Memory:       "512Mi",
				Concurrency:  80,
				MinInstances: 0,
				MaxInstances: 10,
				Timeout:      "300s",
				HealthPath:   "/health",
			},
		},
		Local: &workspace.ProjectLocalConfig{
			CloudRun: &workspace.ProjectLocalCloudRun{
				Port: 3000,
				Env: map[string]string{
					"NODE_ENV": "development",
				},
			},
			GKE: &workspace.ProjectLocalGKE{
				Port: 3000,
				Env: map[string]string{
					"NODE_ENV": "development",
				},
			},
		},
	}

	config.Projects[serviceName] = project

	if err := config.SaveToDir(workspaceRoot); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	fmt.Printf("✓ Created NestJS service: %s\n", serviceName)
	fmt.Printf("  Location: %s\n", serviceDir)
	fmt.Printf("  Registry: %s\n", registry)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. cd %s\n", filepath.Join(servicesPath, serviceName))
	fmt.Printf("  2. npm install\n")
	fmt.Printf("  3. npm run start:dev\n")
	fmt.Printf("  4. forge deploy --env=local\n")

	return nil
}

// Template contents
const nestJSPackageJSON = `{
  "name": "{{.ServiceName}}",
  "version": "1.0.0",
  "description": "{{.ServiceName}} NestJS microservice",
  "private": true,
  "scripts": {
    "build": "nest build",
    "format": "prettier --write \"src/**/*.ts\" \"test/**/*.ts\"",
    "start": "nest start",
    "start:dev": "nest start --watch",
    "start:debug": "nest start --debug --watch",
    "start:prod": "node dist/main",
    "lint": "eslint \"{src,test}/**/*.ts\" --fix",
    "test": "jest",
    "test:watch": "jest --watch",
    "test:cov": "jest --coverage",
    "test:debug": "node --inspect-brk -r tsconfig-paths/register -r ts-node/register node_modules/.bin/jest --runInBand",
    "test:e2e": "jest --config ./test/jest-e2e.json"
  },
  "dependencies": {
    "@nestjs/common": "^10.0.0",
    "@nestjs/core": "^10.0.0",
    "@nestjs/platform-express": "^10.0.0",
    "@nestjs/terminus": "^10.0.0",
    "reflect-metadata": "^0.1.13",
    "rxjs": "^7.8.1"
  },
  "devDependencies": {
    "@nestjs/cli": "^10.0.0",
    "@nestjs/schematics": "^10.0.0",
    "@nestjs/testing": "^10.0.0",
    "@types/express": "^4.17.17",
    "@types/jest": "^29.5.2",
    "@types/node": "^20.3.1",
    "@types/supertest": "^2.0.12",
    "@typescript-eslint/eslint-plugin": "^6.0.0",
    "@typescript-eslint/parser": "^6.0.0",
    "eslint": "^8.42.0",
    "eslint-config-prettier": "^9.0.0",
    "eslint-plugin-prettier": "^5.0.0",
    "jest": "^29.5.0",
    "prettier": "^3.0.0",
    "source-map-support": "^0.5.21",
    "supertest": "^6.3.3",
    "ts-jest": "^29.1.0",
    "ts-loader": "^9.4.3",
    "ts-node": "^10.9.1",
    "tsconfig-paths": "^4.2.0",
    "typescript": "^5.1.3"
  },
  "jest": {
    "moduleFileExtensions": ["js", "json", "ts"],
    "rootDir": "src",
    "testRegex": ".*\\\\.spec\\\\.ts$",
    "transform": {
      "^.+\\\\.(t|j)s$": "ts-jest"
    },
    "collectCoverageFrom": ["**/*.(t|j)s"],
    "coverageDirectory": "../coverage",
    "testEnvironment": "node"
  }
}
`

const nestJSTSConfig = `{
  "compilerOptions": {
    "module": "commonjs",
    "declaration": true,
    "removeComments": true,
    "emitDecoratorMetadata": true,
    "experimentalDecorators": true,
    "allowSyntheticDefaultImports": true,
    "target": "ES2021",
    "sourceMap": true,
    "outDir": "./dist",
    "baseUrl": "./",
    "incremental": true,
    "skipLibCheck": true,
    "strictNullChecks": false,
    "noImplicitAny": false,
    "strictBindCallApply": false,
    "forceConsistentCasingInFileNames": false,
    "noFallthroughCasesInSwitch": false
  }
}
`

const nestJSCLIConfig = `{
  "$schema": "https://json.schemastore.org/nest-cli",
  "collection": "@nestjs/schematics",
  "sourceRoot": "src",
  "compilerOptions": {
    "deleteOutDir": true
  }
}
`

const nestJSESLintConfig = `module.exports = {
  parser: '@typescript-eslint/parser',
  parserOptions: {
    project: 'tsconfig.json',
    tsconfigRootDir: __dirname,
    sourceType: 'module',
  },
  plugins: ['@typescript-eslint/eslint-plugin'],
  extends: [
    'plugin:@typescript-eslint/recommended',
    'plugin:prettier/recommended',
  ],
  root: true,
  env: {
    node: true,
    jest: true,
  },
  ignorePatterns: ['.eslintrc.js'],
  rules: {
    '@typescript-eslint/interface-name-prefix': 'off',
    '@typescript-eslint/explicit-function-return-type': 'off',
    '@typescript-eslint/explicit-module-boundary-types': 'off',
    '@typescript-eslint/no-explicit-any': 'off',
  },
};
`

const nestJSPrettierConfig = `{
  "singleQuote": true,
  "trailingComma": "all"
}
`

const nestJSDockerfile = `FROM node:22-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM node:22-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY --from=builder /app/dist ./dist

ENV NODE_ENV=production
ENV PORT=3000

EXPOSE 3000

CMD ["node", "dist/main"]
`

const nestJSMainTS = "import { NestFactory } from '@nestjs/core';\n" +
	"import { AppModule } from './app.module';\n" +
	"\n" +
	"async function bootstrap() {\n" +
	"  const app = await NestFactory.create(AppModule);\n" +
	"  \n" +
	"  const port = process.env.PORT || 3000;\n" +
	"  await app.listen(port);\n" +
	"  \n" +
	"  console.log(`Application is running on: http://localhost:${port}`);\n" +
	"}\n" +
	"\n" +
	"bootstrap();\n"

const nestJSAppModule = "import { Module } from '@nestjs/common';\n" +
	"import { TerminusModule } from '@nestjs/terminus';\n" +
	"import { AppController } from './app.controller';\n" +
	"import { AppService } from './app.service';\n" +
	"import { HealthController } from './health/health.controller';\n" +
	"\n" +
	"@Module({\n" +
	"  imports: [TerminusModule],\n" +
	"  controllers: [AppController, HealthController],\n" +
	"  providers: [AppService],\n" +
	"})\n" +
	"export class AppModule {}\n"

const nestJSAppController = "import { Controller, Get } from '@nestjs/common';\n" +
	"import { AppService } from './app.service';\n" +
	"\n" +
	"@Controller()\n" +
	"export class AppController {\n" +
	"  constructor(private readonly appService: AppService) {}\n" +
	"\n" +
	"  @Get()\n" +
	"  getHello(): string {\n" +
	"    return this.appService.getHello();\n" +
	"  }\n" +
	"}\n"

const nestJSAppService = "import { Injectable } from '@nestjs/common';\n" +
	"\n" +
	"@Injectable()\n" +
	"export class AppService {\n" +
	"  getHello(): string {\n" +
	"    return 'Hello from {{.ServiceName}}!';\n" +
	"  }\n" +
	"}\n"

const nestJSHealthController = "import { Controller, Get } from '@nestjs/common';\n" +
	"import { HealthCheck, HealthCheckService } from '@nestjs/terminus';\n" +
	"\n" +
	"@Controller('health')\n" +
	"export class HealthController {\n" +
	"  constructor(private health: HealthCheckService) {}\n" +
	"\n" +
	"  @Get()\n" +
	"  @HealthCheck()\n" +
	"  check() {\n" +
	"    return this.health.check([]);\n" +
	"  }\n" +
	"}\n"

const nestJSE2ETest = "import { Test, TestingModule } from '@nestjs/testing';\n" +
	"import { INestApplication } from '@nestjs/common';\n" +
	"import * as request from 'supertest';\n" +
	"import { AppModule } from './../src/app.module';\n" +
	"\n" +
	"describe('AppController (e2e)', () => {\n" +
	"  let app: INestApplication;\n" +
	"\n" +
	"  beforeEach(async () => {\n" +
	"    const moduleFixture: TestingModule = await Test.createTestingModule({\n" +
	"      imports: [AppModule],\n" +
	"    }).compile();\n" +
	"\n" +
	"    app = moduleFixture.createNestApplication();\n" +
	"    await app.init();\n" +
	"  });\n" +
	"\n" +
	"  it('/ (GET)', () => {\n" +
	"    return request(app.getHttpServer())\n" +
	"      .get('/')\n" +
	"      .expect(200)\n" +
	"      .expect('Hello from {{.ServiceName}}!');\n" +
	"  });\n" +
	"\n" +
	"  it('/health (GET)', () => {\n" +
	"    return request(app.getHttpServer())\n" +
	"      .get('/health')\n" +
	"      .expect(200);\n" +
	"  });\n" +
	"});\n"

const nestJSJestE2EConfig = "{\n" +
	"  \"moduleFileExtensions\": [\"js\", \"json\", \"ts\"],\n" +
	"  \"rootDir\": \".\",\n" +
	"  \"testEnvironment\": \"node\",\n" +
	"  \"testRegex\": \".e2e-spec.ts$\",\n" +
	"  \"transform\": {\n" +
	"    \"^.+\\\\.(t|j)s$\": \"ts-jest\"\n" +
	"  }\n" +
	"}\n"

const nestJSHelmValues = `# {{.ServiceName}} - Helm Values
nameOverride: "{{.ServiceName}}"

image:
  repository: {{.Registry}}/{{.ServiceName}}
  tag: "latest"

service:
  port: 80
  targetPort: 3000

resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi

livenessProbe:
  httpGet:
    path: /health
    port: http
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: http
  initialDelaySeconds: 10
  periodSeconds: 5

env:
  - name: NODE_ENV
    value: "production"
  - name: PORT
    value: "3000"
`

const nestJSCloudRunService = `apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: {{.ServiceName}}
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/minScale: "0"
        autoscaling.knative.dev/maxScale: "10"
    spec:
      containerConcurrency: 80
      timeoutSeconds: 300
      containers:
        - name: {{.ServiceName}}
          image: {{.Registry}}/{{.ServiceName}}:latest
          ports:
            - name: http1
              containerPort: 3000
          env:
            - name: NODE_ENV
              value: "production"
            - name: PORT
              value: "3000"
          resources:
            limits:
              cpu: "1"
              memory: "512Mi"
          livenessProbe:
            httpGet:
              path: /health
              port: 3000
`

const nestJSReadme = "# {{.ServiceName}}\n" +
	"\n" +
	"NestJS microservice generated by Forge.\n" +
	"\n" +
	"## Development\n" +
	"\n" +
	"```bash\n" +
	"# Install dependencies\n" +
	"npm install\n" +
	"\n" +
	"# Run in development mode\n" +
	"npm run start:dev\n" +
	"\n" +
	"# Run tests\n" +
	"npm test\n" +
	"\n" +
	"# Build\n" +
	"npm run build\n" +
	"```\n" +
	"\n" +
	"## Deployment\n" +
	"\n" +
	"```bash\n" +
	"# Deploy to local environment\n" +
	"forge deploy --env=local\n" +
	"\n" +
	"# Deploy to dev\n" +
	"forge deploy --env=dev\n" +
	"\n" +
	"# Deploy to production\n" +
	"forge deploy --env=prod\n" +
	"```\n" +
	"\n" +
	"## API Endpoints\n" +
	"\n" +
	"- `GET /` - Hello endpoint\n" +
	"- `GET /health` - Health check\n" +
	"\n" +
	"## Configuration\n" +
	"\n" +
	"Service configuration is managed in `forge.json` at the workspace root.\n" +
	"Environment-specific Helm values are in `deploy/helm/values-{env}.yaml`.\n"
