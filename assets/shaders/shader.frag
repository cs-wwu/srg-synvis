#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(binding = 1) uniform sampler2D texSampler;

layout(location = 0) in vec3 fragColor;
layout(location = 1) in vec3 fragTexCoord;
layout(location = 2) in vec3 worldPos;
layout(location = 3) in float fogFactor;

layout(location = 0) out vec4 outColor;

// Sky color for fog
const vec4 fogColor = vec4(0.7, 0.8, 0.9, 1.0); // Light blue-gray

void main() {
    vec4 texColor = texture(texSampler, fragTexCoord.xy);
    vec4 baseColor = texColor * vec4(fragColor, 1.0);
    
    // Check if the height (y-coordinate) is greater than 0 (then it's water)
    if (fragTexCoord.z > 0.5) {
        // Calculate depth for water color variation
        float waterDepth = 1.0 - fragColor.r * 0.5;  // Use lighting intensity for depth approximation
        
        // Create deep to shallow water gradient
        vec3 deepWater = vec3(0.0, 0.05, 0.2);       // Deep water color (dark blue)
        vec3 shallowWater = vec3(0.0, 0.4, 0.8);     // Shallow water color (light blue)
        vec3 waterColor = mix(deepWater, shallowWater, waterDepth);
        
        // Add specular highlight for water surface
        vec3 lightDir = normalize(ubo.lightDir);
        vec3 viewDir = normalize(-worldPos);
        vec3 reflectDir = reflect(-lightDir, vec3(0.0, 1.0, 0.0));  // Assuming water surface normal is up
        float spec = pow(max(dot(viewDir, reflectDir), 0.0), 32.0);
        vec3 specular = spec * vec3(1.0, 1.0, 1.0) * 0.3;  // White specular highlights
        
        // Add simple wave animation effect (subtle color variation)
        float wave = sin(worldPos.x * 0.05 + worldPos.z * 0.03 + ubo.time * 0.5) * 0.05;
        waterColor += wave;
        
        // Combine colors
        vec4 waterWithSpec = vec4(waterColor + specular, 0.95);  // Slightly transparent
        
        baseColor = waterWithSpec;
    }
    
    // Apply fog - blend between baseColor and fogColor based on fogFactor
    outColor = mix(fogColor, baseColor, fogFactor);
}
