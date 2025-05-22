#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(std140, binding = 0) uniform UBO {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    vec3 lightDir;
    vec3 cameraPos;

    float renderDistance;
    float warningsDisabled;
    float time;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inTexCoord;
layout(location = 2) in vec3 inNormal;

const float AMBIENT = 0.15;       // Ambient light intensity
const float FOG_OFFSET = 1500.0;   // How far to start the fog from the renderDistance

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
    
    // Calculate fog factors based on distance and height
    float fogStart = 3500;  // e.g., 5000 - 1000 = 4000
    float fogEnd = 5000;                 // e.g., 5000

    // Calculate fog factors based on distance and height
    float distance = length(viewPosition.xyz);
    // float heightFactor = smoothstep(0.0, 2000.0, worldPosition.y);

    // Smoother fog transition using dynamic distances
    fogFactor = 1.0 - clamp((distance - fogStart) / (fogEnd - fogStart), 0.0, 1.0);
    fogFactor = pow(fogFactor, 1.5);

    // Make fog more transparent at high elevations
    // fogFactor = mix(fogFactor, 1.0, heightFactor * 0.3);

    fragColor = intense * vec3(1.0);
    fragTexCoord = inTexCoord.xyz;
    worldPos = worldPosition.xyz;
}
