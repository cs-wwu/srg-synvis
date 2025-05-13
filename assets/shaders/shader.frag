#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(binding = 1) uniform sampler2D texSampler;

layout(std140, binding = 0) uniform UBO {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    vec3 lightDir;
    float time;
} ubo;

layout(location = 0) in vec3 fragColor;
layout(location = 1) in vec3 fragTexCoord;
layout(location = 2) in vec3 worldPos;
layout(location = 3) in float fogFactor;

layout(location = 0) out vec4 outColor;

// Improved sky gradient colors
vec4 getSkyColor(float height) {
    // Sky zenith and horizon colors
    vec3 zenithColor = vec3(0.2, 0.4, 0.8);     // Deeper blue at zenith
    vec3 horizonColor = vec3(0.7, 0.85, 1.0);   // Light blue at horizon
    
    // Calculate height factor (0 at horizon, 1 at zenith) 
    // with smoother gradient around horizon
    float skyFactor = smoothstep(0.0, 1.0, height * 2.0);
    
    // Create sky gradient
    return vec4(mix(horizonColor, zenithColor, skyFactor), 1.0);
}

void main() {
    // Process terrain or water
    vec4 baseColor;
    
    if (fragTexCoord.z > 0.5) {
        // WATER RENDERING
        // Calculate depth for water color variation
        float waterDepth = 1.0 - fragColor.r * 0.5;
        
        // Create deep to shallow water gradient
        vec3 deepWater = vec3(0.0, 0.05, 0.2);      // Deep water color
        vec3 shallowWater = vec3(0.0, 0.4, 0.8);    // Shallow water color
        vec3 waterColor = mix(deepWater, shallowWater, waterDepth);
        
        // Add specular highlight for water surface only
        vec3 lightDir = normalize(ubo.lightDir);
        vec3 viewDir = normalize(-worldPos);
        vec3 reflectDir = reflect(-lightDir, vec3(0.0, 1.0, 0.0));
        float spec = pow(max(dot(viewDir, reflectDir), 0.0), 32.0);
        vec3 specular = spec * vec3(1.0, 1.0, 1.0) * 0.3;
        
        // Simple wave animation effect
        float wave = sin(worldPos.x * 0.05 + worldPos.z * 0.03 + ubo.time * 1.0) * 0.05;
        waterColor += wave;
        
        // Combine colors for water only
        baseColor = vec4(waterColor + specular, 0.95);
    } else {
        // TERRAIN RENDERING - preserve original appearance
        vec4 texColor = texture(texSampler, fragTexCoord.xy);
        if (texColor[3] < 1.0) {
            baseColor = vec4(0.13, 0.55, 0.13, 1.0) * vec4(fragColor, 1.0); // Grass/forest green
        } else {
            baseColor = texColor * vec4(fragColor, 1.0);
        }
    }
    
    // Get sky gradient color - use more vibrant colors
    vec3 zenithColor = vec3(0.1, 0.3, 0.8);     // Deeper blue at zenith
    vec3 horizonColor = vec3(0.7, 0.85, 1.0);   // Light blue at horizon
    
    // Calculate view direction in world space
    vec3 viewDir = normalize(worldPos);
    // Convert to "upness" factor - how much we're looking up
    float upFactor = max(0.0, viewDir.y);
    
    // Create sky gradient - force stronger gradient transition
    vec4 skyColor = vec4(mix(horizonColor, zenithColor, upFactor), 1.0);
    
    // Make sure fog is applied correctly
    // At fogFactor = 0 (far away), use pure sky color
    // At fogFactor = 1 (nearby), use pure terrain/water color
    outColor = mix(skyColor, baseColor, fogFactor);
}
