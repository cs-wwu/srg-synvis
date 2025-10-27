#version 450

layout(lines) in;
layout(triangle_strip, max_vertices = 4) out;

layout(binding = 0) uniform UniformBufferObject {
    mat4 model;
    mat4 view;
    mat4 projection;
} ubo;

layout(location = 0) in vec3 inColor[];
layout(location = 1) in float inLowerAlt[];
layout(location = 2) in float inUpperAlt[];

layout(location = 0) out vec3 fragColor;

void main() {
    // Get the two vertices of the line (edge of polygon)
    vec4 p0 = gl_in[0].gl_Position; // Bottom of edge
    vec4 p1 = gl_in[1].gl_Position; // Bottom of next edge
    
    float lower0 = inLowerAlt[0];
    float upper0 = inUpperAlt[0];
    float lower1 = inLowerAlt[1];
    float upper1 = inUpperAlt[1];
    
    // Extrude into a quad (4 vertices forming 2 triangles)
    
    // Bottom-left
    gl_Position = ubo.projection * ubo.view * vec4(p0.xyz + vec3(0, lower0, 0), 1.0);
    fragColor = inColor[0];
    EmitVertex();
    
    // Bottom-right
    gl_Position = ubo.projection * ubo.view * vec4(p1.xyz + vec3(0, lower1, 0), 1.0);
    fragColor = inColor[1];
    EmitVertex();
    
    // Top-left
    gl_Position = ubo.projection * ubo.view * vec4(p0.xyz + vec3(0, upper0, 0), 1.0);
    fragColor = inColor[0];
    EmitVertex();
    
    // Top-right
    gl_Position = ubo.projection * ubo.view * vec4(p1.xyz + vec3(0, upper1, 0), 1.0);
    fragColor = inColor[1];
    EmitVertex();
    
    EndPrimitive();
}