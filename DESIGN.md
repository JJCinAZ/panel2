# Design System Document: The Architectural Monolith
 
## 1. Overview & Creative North Star: "The Digital Letterpress"
The Creative North Star for this design system is **"The Digital Letterpress."** Because e-ink technology thrives on high contrast and suffers with motion or gradients, we embrace the medium's limitations as a premium stylistic choice. We move away from "tablet UI" and toward the world of high-end architectural signage and avant-garde editorial print.
 
This system rejects the "gray-scale" trap. Instead of muddy transitions, we use intentional asymmetry and razor-sharp linework to create a sense of permanence and authority. The UI should feel less like a screen and more like a custom-etched piece of slate or a masterfully typeset broadsheet.
 
## 2. Colors: The Power of Binary
In a strictly monochromatic environment, color is not a spectrum; it is a choice of "Presence" vs. "Void."
 
### The Palette (Token Implementation)
*   **Primary (`#000000`):** Used for all structural linework, primary text, and "active" architectural elements.
*   **Surface (`#FFFFFF` / `#f9f9f9`):** The stark canvas. While the token says `#f9f9f9`, for e-ink, this is treated as a "full clear" state to prevent ghosting.
*   **On-Surface (`#1a1c1c`):** High-density black for maximum legibility on small 7-inch displays.
 
### The "No-Gray" Rule
E-ink displays struggle with "refreshing" shades of gray, often leading to ghosting. Therefore:
*   **Prohibit Gradients:** All backgrounds must be solid `#ffffff` or solid `#000000`.
*   **The Contrast Rule:** Never place `on_surface_variant` (gray) on a dark background. Hierarchy must be achieved through **Scale** and **Weight**, not color value.
 
### Surface Hierarchy & Nesting
Depth is achieved through "The Inversion Principle." To nest a container, do not use a shadow; instead, invert the color space. A black "Suite Number" box (`primary`) sits on the white background (`surface`), creating a physical "punched-out" effect similar to a letterpress.
 
## 3. Typography: Editorial Authority
The typography is the soul of this system. We pair a high-fashion serif with a utilitarian sans-serif to bridge the gap between "Hospitality" and "Office Efficiency."
 
*   **The Display Serif (Noto Serif):** Used for the **Suite Number** and **Tenant Name**. It conveys a premium, established feel.
    *   *Display-LG (3.5rem):* Reserved for Suite Numbers.
    *   *Headline-LG (2rem):* Reserved for Primary Tenant Names.
*   **The Utility Sans (Public Sans & Inter):** Used for wayfinding, instructions, and secondary labels.
    *   *Title-MD (1.125rem):* For secondary occupants or floor levels.
    *   *Label-SM (0.6875rem):* For functional metadata (e.g., "Available until 2:00 PM").
 
**Hierarchy Strategy:** Use `uppercase` for sans-serif labels to create a "monumental" architectural feel, contrasting with the fluid, elegant curves of the Noto Serif headlines.
 
## 4. Elevation & Depth: The Architectural Blueprint
Since we cannot use shadows or blurs, depth is conveyed through **Linework and Framing.**
 
*   **The Layering Principle:** Treat the 800x480 canvas like a blueprint. Use "Tonal Inversion" to lift elements. A black header bar at the top of the display creates a heavy "anchor" for the eye, while the white space below feels expansive.
*   **The Structural Border:** Replace "Ambient Shadows" with **Double-Lines.** To signify a high-priority area (like a "Meeting in Progress" status), wrap the container in a 2px `primary` border with a 2px internal offset (padding), creating a "frame within a frame."
*   **Architectural Accents:** Use 45-degree diagonal hatches or solid 10px vertical bars to denote "State." A solid black vertical bar on the far left edge of the screen indicates an "Occupied" status—clear, bold, and visible from 20 feet down a hallway.
 
## 5. Components: Precision Elements
 
### The Header (Suite Identity)
The core component of the office suite display.
*   **Layout:** Asymmetric. Suite number (`Display-LG`) is positioned in the top-right, right-aligned. The Tenant Name (`Headline-LG`) is positioned bottom-left. This "diagonal tension" makes the small 7-inch screen feel larger and more custom.
*   **Styling:** Separated by a single 2px horizontal rule (`primary`) that stops exactly 40px from the screen edge.
 
### Buttons (Touch Interactions)
*   **Primary:** Solid black background (`primary`) with white text (`on_primary`). Sharp 0px corners.
*   **Secondary:** White background with a 2px solid black border.
*   **States:** On e-ink, "Pressed" states should stay inverted (White text on Black) until the screen refreshes to avoid "flashing" artifacts.
 
### Lists & Schedules
*   **Forbid Dividers:** Use vertical white space (`1.5rem`) to separate upcoming meetings.
*   **Time-Blocks:** Use a "Time-Line" component—a 1px vertical line that connects list items, creating a visual "track" for the eye to follow.
 
### Architectural Decorations
*   **The "Corner Frame":** For premium suites, use "L-shaped" corner accents (20px x 20px) in the top-left and bottom-right corners of the screen to "lock" the content and provide a sense of luxury stationery.
 
## 6. Do's and Don'ts
 
### Do:
*   **DO** use 0px border-radii for everything. Sharp corners emphasize the "Architectural" feel.
*   **DO** leave at least 40px of "Safe Zone" padding around the edges of the 800x480 display.
*   **DO** use "White-on-Black" (Inversion) for the most important information on the screen.
 
### Don't:
*   **DON'T** use gray-scale anti-aliasing on lines. Ensure all lines are snapped to whole pixels to keep the e-ink display crisp.
*   **DON'T** use dividers that span the full width of the screen. Instead, use "broken rules" (lines that stop 25% of the way) to create a more sophisticated, editorial look.
*   **DON'T** use icons unless they are strictly "Solid" or "Bold" styles. Thin-line icons will disappear or appear "broken" on many e-ink panels. Use text labels in `Label-MD` (uppercase) instead.

