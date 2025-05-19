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

// Better hash function for more randomness
float hash(vec2 p) {
    p = 50.0 * fract(p * 0.3183099);
    return fract(p.x * p.y * (p.x + p.y));
}

// Simple noise - adjusted for finer detail
float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    
    float n00 = hash(i);
    float n01 = hash(i + vec2(0.0, 1.0));
    float n10 = hash(i + vec2(1.0, 0.0));
    float n11 = hash(i + vec2(1.0, 1.0));
    
    return mix(mix(n00, n10, f.x), mix(n01, n11, f.x), f.y);
}

// Fine-scale noise for terrain detail
float detailNoise(vec2 p) {
    return noise(p * 0.3);
}

vec4 getSkyColor(vec3 viewDir) {
    // Rich blue zenith
    vec3 zenithColor = vec3(0.1, 0.3, 0.8);  
    // Light blue horizon
    vec3 horizonColor = vec3(0.7, 0.85, 1.0);
    
    // Calculate upward direction factor
    float upFactor = max(0.0, viewDir.y);
    
    // Apply non-linear gradient
    float skyGradient = pow(upFactor, 0.4);
    
    return vec4(mix(horizonColor, zenithColor, skyGradient), 1.0);
}

void main() {
    // Process terrain or water
    vec4 baseColor;
    
    // Calculate steepness based on the Y component of the normal
    // A value of 0 means the surface is vertical, 1 means it's flat
    vec3 normNormal = normalize(fragNormal);
    float steepness = abs(normNormal.y);
    
    if (fragTexCoord.z > 0.5) {
        // Water rendering
        float waterDepth = 1.0 - fragColor.r * 0.5;
        
        vec3 deepWater = vec3(0.0, 0.05, 0.2);      
        vec3 shallowWater = vec3(0.0, 0.4, 0.8);
        vec3 waterColor = mix(deepWater, shallowWater, waterDepth);
        
        // Add specular reflection to water
        vec3 lightDir = normalize(ubo.lightDir);
        vec3 viewDir = normalize(-worldPos);
        vec3 reflectDir = reflect(-lightDir, vec3(0.0, 1.0, 0.0));
        float spec = pow(max(dot(viewDir, reflectDir), 0.0), 32.0);
        vec3 specular = spec * vec3(1.0, 1.0, 1.0) * 0.3;
        
        // Wave animation
        float wave1 = sin(worldPos.x * 0.05 + worldPos.z * 0.03 + ubo.time * 1.0) * 0.05;
        waterColor += wave1;
        
        baseColor = vec4(waterColor + specular, 0.95);
    } else {
        // Render terrain with varied green forest, gray rock, and white snow
        vec4 texColor = texture(texSampler, fragTexCoord.xy);
        
        // Define colors for different terrain types
        vec3 green1 = vec3(0.05, 0.40, 0.05); // Darker green
        vec3 green2 = vec3(0.10, 0.50, 0.10); // Medium green
        vec3 green3 = vec3(0.16, 0.55, 0.12); // Lighter green
        vec3 green4 = vec3(0.20, 0.60, 0.15); // Bright green
        vec3 green5 = vec3(0.13, 0.45, 0.08); // Another medium variation
        
        vec3 rockColor = vec3(0.65, 0.63, 0.60); // Light gray rock
        vec3 snowColor = vec3(0.9, 0.9, 0.95);   // Snow
        
        // Generate fine noise values at different frequencies
        float n1 = detailNoise(worldPos.xz);
        float n2 = detailNoise(worldPos.xz * 1.7 + vec2(42.5, 13.2));
        float n3 = detailNoise(worldPos.xz * 2.3 + vec2(11.9, 78.6));
        float n4 = detailNoise(worldPos.xz * 3.1 + vec2(87.2, 33.4));
        
        // Create a random green color using weighted noise
        float greenSelector = n1 * 0.5 + n2 * 0.3 + n3 * 0.15 + n4 * 0.05;
        
        // Create a random blend between the green colors
        vec3 forestColor;
        if (greenSelector < 0.2) {
            forestColor = green1;
        } else if (greenSelector < 0.4) {
            forestColor = green5;
        } else if (greenSelector < 0.6) {
            forestColor = green2;
        } else if (greenSelector < 0.8) {
            forestColor = green3;
        } else {
            forestColor = green4;
        }
        
        // Add small-scale variation
        float detailVar = detailNoise(worldPos.xz * 5.0) * 0.1 - 0.05;
        forestColor = forestColor * (1.0 + detailVar);
        
        // Calculate mix factor for rock/forest based on steepness
        // Lower slopeFactor values (steeper terrain) will show more rock
        float slopeFactor = smoothstep(0.0, 0.5, steepness); // Steeper threshold for rock
        vec3 terrainColor = mix(rockColor, forestColor, slopeFactor);
        
        // Add height-based coloring - snow on peaks
        float heightFactor = smoothstep(1500.0, 2000.0, worldPos.y);
        terrainColor = mix(terrainColor, snowColor, heightFactor);
        
        // Add slight variation to rock color based on direction (warmer on south facing)
        if (slopeFactor < 0.5) { // Only for rocky areas
            // Get the slope direction (which way is it facing)
            vec2 slopeDir = normalize(normNormal.xz);
            
            // Check if facing south (roughly +Z in your world space)
            float southness = slopeDir.y; // More positive = more south-facing
            
            // Add subtle warm tint to south-facing rocks, cool tint to north-facing
            terrainColor += vec3(southness * 0.05, southness * 0.03, -southness * 0.05) * (1.0 - slopeFactor * 2.0);
        }
        
        // Use texture if available
        if (texColor.a > 0.5) {
            baseColor = texColor * vec4(fragColor, 1.0); 
        } else {
            // Apply lighting from vertex shader
            baseColor = vec4(terrainColor * fragColor, 1.0);
        }
    }
    
    // Calculate view direction for sky/fog
    vec3 viewDir = normalize(worldPos);
    
    // Get enhanced sky color
    vec4 skyColor = getSkyColor(viewDir);
    
    // Adjust fog based on height - higher terrain has less fog
    float heightFogFactor = smoothstep(500.0, 2000.0, worldPos.y);
    float enhancedFogFactor = fogFactor * (1.0 - heightFogFactor * 0.3);
    
    // Final blending of terrain/water with sky
    outColor = mix(skyColor, baseColor, enhancedFogFactor);
}
