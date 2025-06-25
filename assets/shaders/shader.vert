#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(std140, binding = 0) uniform UBO {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    mat4 planeDirection;
    vec3 lightDir;
    float renderDistance;
    vec3 cameraPos;
    float time;
    float disableWarnings;
    float disableFog;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inTexCoord;
layout(location = 2) in vec3 inNormal;

const float AMBIENT = 0.15;      // Ambient light intensity
const float FOG_OFFSET = 0.75;   // How far to start the fog from the renderDistance

layout(location = 0) out vec3 fragColor;
layout(location = 1) out vec3 fragTexCoord;
layout(location = 2) out vec3 worldPos;
layout(location = 3) out float fogFactor;
layout(location = 4) out vec3 fragNormal;

out gl_PerVertex {
    vec4 gl_Position;
};

void main() 
{
    vec4 worldPosition = ubo.model * vec4(inPosition, 1.0);
    vec4 viewPosition = ubo.view * worldPosition;
    gl_Position = ubo.proj * viewPosition;

    // Transform normal to world space
    vec3 worldNormal = normalize(mat3(ubo.normalM) * inNormal);
    fragNormal = worldNormal;
    
    // Calculate lighting based on surface normal and light direction
    vec3 lightDir = normalize(ubo.lightDir);
    float diffuse = max(dot(worldNormal, lightDir), 0.0);
    float intense = AMBIENT + diffuse * 0.85;

    // Calculate fog factor
    if (ubo.disableFog > 0.5) {
        // Fog disabled - no fog blending
        fogFactor = 1.0;
    } else {
        // Fog enabled - calculate fog based on distance and height
        float fogStart = ubo.renderDistance * FOG_OFFSET;
        float distance = length(viewPosition.xyz);
        float heightFactor = smoothstep(0.0, 2000.0, worldPosition.y);

        // Calculate base fog factor
        fogFactor = 1.0 - clamp((distance - fogStart) / (ubo.renderDistance - fogStart), 0.0, 1.0);
        fogFactor = pow(fogFactor, 1.5);

        // Reduce fog at high elevations
        fogFactor = mix(fogFactor, 1.0, heightFactor * 0.3);
    }

    fragColor = intense * vec3(1.0);
    fragTexCoord = inTexCoord.xyz;
    worldPos = worldPosition.xyz;
}
