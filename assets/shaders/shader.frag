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
layout(location = 4) in vec3 fragNormal;

layout(location = 0) out vec4 outColor;

// Enhanced sky gradient function
vec4 getSkyColor(vec3 viewDir) {
    // Deeper, more vibrant blue for zenith
    vec3 zenithColor = vec3(0.1, 0.3, 0.8);  
    // Lighter blue with a hint of haze for horizon
    vec3 horizonColor = vec3(0.7, 0.85, 1.0);
    
    // Calculate upward direction factor (how much we're looking up)
    float upFactor = max(0.0, viewDir.y);
    
    // Apply non-linear gradient for more dramatic effect
    float skyGradient = pow(upFactor, 0.4);
    
    // Create the gradient
    return vec4(mix(horizonColor, zenithColor, skyGradient), 1.0);
}

void main() {
    // Process terrain or water
    vec4 baseColor;
    
    // Get terrain steepness from normal (closer to 0 for steep surfaces)
    float steepness = max(0.0, fragNormal.y);
    
    if (fragTexCoord.z > 0.5) {
        // WATER RENDERING
        float waterDepth = 1.0 - fragColor.r * 0.5;
        
        // Better water color gradient - deeper blue
        vec3 deepWater = vec3(0.0, 0.05, 0.2);      
        vec3 shallowWater = vec3(0.0, 0.4, 0.8);
        vec3 waterColor = mix(deepWater, shallowWater, waterDepth);
        
        // Improved specular for water
        vec3 lightDir = normalize(ubo.lightDir);
        vec3 viewDir = normalize(-worldPos);
        vec3 reflectDir = reflect(-lightDir, vec3(0.0, 1.0, 0.0));
        float spec = pow(max(dot(viewDir, reflectDir), 0.0), 32.0);
        vec3 specular = spec * vec3(1.0, 1.0, 1.0) * 0.3;
        
        // Simple wave animation effect
        float wave = sin(worldPos.x * 0.05 + worldPos.z * 0.03 + ubo.time * 1.0) * 0.05;
        waterColor += wave;
        
        baseColor = vec4(waterColor + specular, 0.95);
    } else {
        // TERRAIN RENDERING
        vec4 texColor = texture(texSampler, fragTexCoord.xy);
        
        // Basic terrain color implementation
        vec3 grassColor = vec3(0.13, 0.55, 0.13); // Base green
        vec3 rockColor = vec3(0.6, 0.6, 0.6);     // Gray for steep surfaces
        vec3 snowColor = vec3(0.9, 0.9, 0.95);    // White/light gray for high areas
        
        // Mix based on steepness - more gray on steep slopes
        vec3 terrainColor = mix(rockColor, grassColor, steepness);
        
        // Add height-based coloring (higher = more snow/lighter)
        float heightFactor = smoothstep(800.0, 2000.0, worldPos.y);
        terrainColor = mix(terrainColor, snowColor, heightFactor);
        
        // Add some randomness to the color
        float noise = fract(sin(dot(fragTexCoord.xy, vec2(12.9898, 78.233))) * 43758.5453) * 0.1 - 0.05;
        terrainColor += vec3(noise);
        
        if (texColor.a > 0.5) {
            baseColor = texColor * vec4(fragColor, 1.0); 
        } else {
            baseColor = vec4(terrainColor * fragColor, 1.0);
        }
    }
    
    // Calculate view direction for sky/fog
    vec3 viewDir = normalize(worldPos);
    
    // Get enhanced sky color
    vec4 skyColor = getSkyColor(viewDir);
    
    // Enhanced fog calculation with height factor
    float heightFogFactor = smoothstep(500.0, 2000.0, worldPos.y);
    float enhancedFogFactor = fogFactor * (1.0 - heightFogFactor * 0.3);
    
    // Final blending of terrain/water with sky
    outColor = mix(skyColor, baseColor, enhancedFogFactor);
}
