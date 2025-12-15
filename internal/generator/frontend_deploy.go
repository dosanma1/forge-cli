package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/workspace"
)

// generateEnvironmentFiles creates environment.ts files for different environments
func (g *FrontendGenerator) generateEnvironmentFiles(appDir, appName, deploymentTarget string) error {
	envDir := filepath.Join(appDir, "src", "environments")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environments directory: %w", err)
	}

	// environment.ts (local development)
	envContent := `export const environment = {
  production: false,
  apiUrl: 'http://localhost:8080/api',
  deployment: '` + deploymentTarget + `'
};
`
	if err := os.WriteFile(filepath.Join(envDir, "environment.ts"), []byte(envContent), 0644); err != nil {
		return err
	}

	// environment.dev.ts
	envDevContent := `export const environment = {
  production: false,
  apiUrl: 'https://api-dev.example.com/api',
  deployment: '` + deploymentTarget + `'
};
`
	if err := os.WriteFile(filepath.Join(envDir, "environment.dev.ts"), []byte(envDevContent), 0644); err != nil {
		return err
	}

	// environment.prod.ts
	envProdContent := `export const environment = {
  production: true,
  apiUrl: 'https://api.example.com/api',
  deployment: '` + deploymentTarget + `'
};
`
	if err := os.WriteFile(filepath.Join(envDir, "environment.prod.ts"), []byte(envProdContent), 0644); err != nil {
		return err
	}

	fmt.Println("  ✓ Generated environment files")
	return nil
}

// generateDeploymentConfig generates deployment configuration based on target
func (g *FrontendGenerator) generateDeploymentConfig(workspaceDir, appName, deploymentTarget string, config *workspace.Config) error {
	switch deploymentTarget {
	case "firebase":
		return g.generateFirebaseConfig(workspaceDir, appName, config)
	case "gke":
		return g.generateGKEConfig(workspaceDir, appName)
	case "cloudrun":
		return g.generateCloudRunConfig(workspaceDir, appName)
	default:
		return fmt.Errorf("unknown deployment target: %s", deploymentTarget)
	}
}

// generateFirebaseConfig generates Firebase hosting configuration
func (g *FrontendGenerator) generateFirebaseConfig(workspaceDir, appName string, config *workspace.Config) error {
	// Put Firebase config in the app directory (self-contained)
	appDir := filepath.Join(workspaceDir, "frontend", "apps", appName)

	// Get project ID from config or use default
	projectID := "your-project-id"
	if config != nil && config.Workspace.GCP != nil && config.Workspace.GCP.ProjectID != "" {
		projectID = config.Workspace.GCP.ProjectID
	}

	// Check if .firebaserc exists
	firebasercPath := filepath.Join(appDir, ".firebaserc")
	firebaseExists := false
	if _, err := os.Stat(firebasercPath); err == nil {
		firebaseExists = true
	}

	if !firebaseExists {
		// Create new .firebaserc with multi-site support
		firebasercContent := `{
  "projects": {
    "default": "` + projectID + `"
  },
  "targets": {
    "` + projectID + `": {
      "hosting": {
        "` + appName + `": [
          "` + appName + `"
        ]
      }
    }
  }
}
`
		if err := os.WriteFile(firebasercPath, []byte(firebasercContent), 0644); err != nil {
			return err
		}

		// Create firebase.json with hosting config
		firebaseJsonPath := filepath.Join(appDir, "firebase.json")
		firebaseJsonContent := `{
  "hosting": [
    {
      "target": "` + appName + `",
      "public": "dist",
      "ignore": [
        "firebase.json",
        "**/.*",
        "**/node_modules/**"
      ],
      "rewrites": [
        {
          "source": "**",
          "destination": "/index.html"
        }
      ]
    }
  ]
}
`
		if err := os.WriteFile(firebaseJsonPath, []byte(firebaseJsonContent), 0644); err != nil {
			return err
		}
	} else {
		// TODO: Update existing .firebaserc and firebase.json to add new site
		fmt.Println("  ℹ️  Firebase config exists, please manually add hosting target for " + appName)
	}

	fmt.Printf("  ✓ Generated Firebase configuration (target: %s)\n", appName)
	return nil
}

// generateGKEConfig generates Kubernetes/Helm configuration
func (g *FrontendGenerator) generateGKEConfig(workspaceDir, appName string) error {
	deployDir := filepath.Join(workspaceDir, "frontend", "projects", appName, "deploy", "helm")
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		return err
	}

	// Create values.yaml for frontend Helm chart
	valuesContent := `# Helm values for ` + appName + ` frontend
image:
  repository: gcr.io/your-project/` + appName + `
  tag: latest
  pullPolicy: IfNotPresent

replicaCount: 2

service:
  type: ClusterIP
  port: 80

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: ` + appName + `.example.com
      paths:
        - path: /
          pathType: Prefix
`
	valuesPath := filepath.Join(deployDir, "values.yaml")
	if err := os.WriteFile(valuesPath, []byte(valuesContent), 0644); err != nil {
		return err
	}

	fmt.Println("  ✓ Generated GKE/Helm configuration")
	return nil
}

// generateCloudRunConfig generates Cloud Run configuration
func (g *FrontendGenerator) generateCloudRunConfig(workspaceDir, appName string) error {
	deployDir := filepath.Join(workspaceDir, "frontend", "projects", appName, "deploy", "cloudrun")
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		return err
	}

	// Create service.yaml for Cloud Run
	serviceContent := `apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: ` + appName + `
spec:
  template:
    spec:
      containers:
        - image: gcr.io/your-project/` + appName + `:latest
          ports:
            - containerPort: 8080
          resources:
            limits:
              memory: 512Mi
              cpu: 1000m
`
	servicePath := filepath.Join(deployDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}

	// Create nginx Dockerfile
	dockerfileContent := `FROM nginx:alpine
COPY dist/` + appName + ` /usr/share/nginx/html
COPY deploy/cloudrun/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
`
	dockerfilePath := filepath.Join(deployDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return err
	}

	// Create nginx.conf
	nginxContent := `server {
    listen 8080;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
`
	nginxPath := filepath.Join(deployDir, "nginx.conf")
	if err := os.WriteFile(nginxPath, []byte(nginxContent), 0644); err != nil {
		return err
	}

	fmt.Println("  ✓ Generated Cloud Run configuration")
	return nil
}
