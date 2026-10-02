#version 450
void main() {
    vec2 pos[6] = vec2[](
        vec2(-1, -1), vec2(1, -1), vec2(1,  1),
        vec2(-1, -1), vec2(1,  1), vec2(-1, 1)
    );
    gl_Position = vec4(pos[gl_VertexIndex], 0.0, 1.0);
}