# PSIL Examples

This directory contains example programs demonstrating PSIL's features.

## Running Examples

```bash
# Build PSIL first
go build ./cmd/psil

# Run any example
./psil examples/hello.psil
./psil examples/fibonacci.psil
./psil examples/shaders.psil
./psil examples/turtle.psil
```

## Basic Examples

| File | Description |
|------|-------------|
| `hello.psil` | Hello World - basic output |
| `fibonacci.psil` | Fibonacci sequence with recursion |
| `factorial.psil` | Factorial using `linrec` combinator |
| `stdlib.psil` | Standard library definitions |

## Shader Examples (`shaders.psil`)

PSIL's graphics system allows shader-style programming where a quotation is applied to each pixel.

| Shader | Output |
|--------|--------|
| **Gradient** - XY color mapping | ![Gradient](../docs/images/gradient.png) |
| **Stripes** - Horizontal bands | ![Stripes](../docs/images/stripes.png) |
| **Checkerboard** - Classic pattern | ![Checker](../docs/images/checker.png) |
| **Radial** - Distance from center | ![Radial](../docs/images/radial.png) |
| **Plasma** - Demoscene effect | ![Plasma](../docs/images/plasma.png) |
| **Sphere** - 3D SDF rendering | ![Sphere](../docs/images/sphere.png) |

## Turtle Graphics (`turtle.psil`)

Logo-style turtle graphics for L-systems, fractals, and generative art.

### Basic Shapes

| Shape | Code | Output |
|-------|------|--------|
| **Square** | `4 [50 fd 90 rt] times` | ![Square](../docs/images/turtle-square.png) |
| **Star** | `5 [60 fd 144 rt] times` | ![Star](../docs/images/turtle-star.png) |
| **Hexagon** | `6 [40 fd 60 rt] times` | ![Hexagon](../docs/images/turtle-hexagon.png) |

### Spirals and Patterns

| Pattern | Description | Output |
|---------|-------------|--------|
| **Spiral** | Expanding spiral with increasing step size | ![Spiral](../docs/images/turtle-spiral.png) |
| **Nested Squares** | Rotated squares with rainbow colors | ![Nested](../docs/images/turtle-nested.png) |
| **Colorful Circles** | 12 overlapping colored circles | ![Circles](../docs/images/turtle-circles.png) |

### L-Systems and Fractals

| Fractal | Description | Output |
|---------|-------------|--------|
| **Koch Curve** | Classic snowflake fractal segment | ![Koch](../docs/images/turtle-koch.png) |
| **Sierpinski** | Triangle outline | ![Sierpinski](../docs/images/turtle-sierpinski.png) |
| **Simple Tree** | Y-shaped branching tree | ![Tree](../docs/images/turtle-tree.png) |
| **Dragon Curve** | Dragon fractal approximation | ![Dragon](../docs/images/turtle-dragon.png) |

## Turtle Commands Reference

| Command | Stack Effect | Description |
|---------|--------------|-------------|
| `turtle` | `image -> turtle` | Create turtle at center |
| `fd` | `turtle n -> turtle` | Move forward n pixels |
| `bk` | `turtle n -> turtle` | Move backward n pixels |
| `rt` | `turtle n -> turtle` | Turn right n degrees |
| `lt` | `turtle n -> turtle` | Turn left n degrees |
| `pu` | `turtle -> turtle` | Pen up (stop drawing) |
| `pd` | `turtle -> turtle` | Pen down (start drawing) |
| `pencolor` | `turtle r g b -> turtle` | Set pen color (0-255) |
| `setxy` | `turtle x y -> turtle` | Move to position |
| `setheading` | `turtle n -> turtle` | Set heading (degrees) |
| `home` | `turtle -> turtle` | Return to center |
| `turtle-img` | `turtle -> image` | Extract canvas |

## Graphics Commands Reference

| Command | Stack Effect | Description |
|---------|--------------|-------------|
| `img-new` | `w h -> image` | Create new image |
| `img-fill` | `image r g b -> image` | Fill with color |
| `img-setpixel` | `image x y r g b -> image` | Set pixel |
| `img-render` | `image [shader] -> image` | Apply shader to all pixels |
| `img-save` | `image filename -> image` | Save as PNG |
