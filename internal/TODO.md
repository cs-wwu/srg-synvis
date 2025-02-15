# Interal TODOs

These are the internal system TODOs.

## TODO

### Cooper

- [ ] Simplify chunk logic
- [ ] Add capacity to render 4 maps at once
  - [ ] Find max memory based on LODs and distance
  - [ ] Increase memory capacity
  - [ ] If map do not exist, load water square (stitch to corners)
  - [ ] Add LOD stitching
  - [ ] Add dynamic map loading
- [ ] Add dynamic texture loading per map (kyle/roads)
  - [ ] Fix texture striping
- [ ] Simplify frustum logic
  - [ ] Move frustum culling from app to TerrainManager
  - [ ] Test performance difference
- [ ] Change Camera to use Quaternions (sensors?)

### Anyone

- [ ] Move `asche` changes to a custom pullable public branch
  - [ ] UI Application in `asche`
