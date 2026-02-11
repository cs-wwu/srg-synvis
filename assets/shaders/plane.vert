#version 450

layout(std140, binding = 0) uniform UBO {
    dmat4 model;
    dmat4 view;
    mat4 proj;
    mat4 normalM;
    dmat4 planeDirection;
    vec3 lightDir;
    float renderDistance;
    dvec3 cameraPos;
    float time;
    float disableWarnings;
    float disableFog;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec2 inTexCoord;
layout(location = 2) in vec3 inNormal;

layout(location = 0) out vec3 fragColor;
layout(location = 1) out vec2 fragTexCoord;
layout(location = 2) out vec3 fragNormal;
layout(location = 3) out vec3 worldPos;

const float AMBIENT = 0.15; // Ambient light intensity

void main() {
    dvec4 worldPosition = ubo.model * dvec4(inPosition, 1.0);
    dvec4 viewPosition = ubo.planeDirection * worldPosition;
    gl_Position = ubo.proj * vec4(viewPosition);

    // Transform normal to world space
    vec3 worldNormal = normalize(mat3(ubo.planeDirection) * mat3(ubo.normalM) * inNormal);
    fragNormal = worldNormal;

    // Calculate lighting based on surface normal and light direction
    vec3 lightDir = normalize(ubo.lightDir);
    float diffuse = max(dot(worldNormal, lightDir), 0.0);
    float intense = AMBIENT + diffuse * 0.85;

    fragColor = intense * vec3(1.0);
    fragTexCoord = inTexCoord;
    worldPos = vec3(worldPosition.xyz);
}
