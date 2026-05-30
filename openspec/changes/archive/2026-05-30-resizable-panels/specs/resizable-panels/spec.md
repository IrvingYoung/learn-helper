## ADDED Requirements

### Requirement: Users can resize panels by dragging dividers
The system SHALL display draggable dividers between the three layout panels (left sidebar, center chat, right viewer). Users SHALL be able to drag these dividers horizontally to resize adjacent panels.

#### Scenario: Drag right divider to make chat panel wider
- **WHEN** user drags the divider between the chat panel and page viewer to the right
- **THEN** the chat panel width increases and the page viewer width decreases

#### Scenario: Drag left divider to make sidebar wider
- **WHEN** user drags the divider between the sidebar and chat panel to the left
- **THEN** the sidebar width increases and the chat panel width decreases

### Requirement: Panel sizes persist across sessions
The system SHALL persist panel width proportions to browser localStorage. When the user reloads the page or returns in a new session, the panel sizes SHALL be restored to their last saved state.

#### Scenario: Panel sizes restored after page reload
- **WHEN** user resizes panels and then reloads the page
- **THEN** the panels appear at the same proportional sizes as before the reload

#### Scenario: Default sizes for new users
- **WHEN** a user visits the app for the first time with no saved preferences
- **THEN** panels display at default sizes (left 20%, center 50%, right 30%)

### Requirement: Minimum panel width constraints
The system SHALL enforce minimum width constraints on each panel to prevent panels from becoming unusably small. The left sidebar minimum SHALL be 150px, the center chat minimum SHALL be 300px, and the right viewer minimum SHALL be 200px.

#### Scenario: Panel cannot be resized below minimum
- **WHEN** user drags a divider attempting to make a panel smaller than its minimum width
- **THEN** the panel stops shrinking at its minimum width and the divider does not move further

### Requirement: Divider visual feedback
The system SHALL provide visual feedback on the resize dividers. Dividers SHALL display a hover state indicating they are interactive, and a drag state during active resizing.

#### Scenario: Hover over divider
- **WHEN** user hovers the cursor over a divider
- **THEN** the divider displays a visual highlight and the cursor changes to a col-resize indicator

#### Scenario: Dragging a divider
- **WHEN** user is actively dragging a divider
- **THEN** the divider remains visually highlighted and panels resize smoothly in real-time

### Requirement: Existing collapse/expand behavior preserved
The system SHALL preserve the existing collapse/expand toggle functionality for the left sidebar and right viewer panels. When a panel is collapsed, its divider SHALL also collapse. When expanded, the panel SHALL restore to its previous size.

#### Scenario: Collapse right panel
- **WHEN** user clicks the toggle to collapse the right viewer panel
- **THEN** the right panel and its right-side divider collapse to zero width

#### Scenario: Expand previously resized panel
- **WHEN** user collapses the right panel, resizes other panels, then expands the right panel again
- **THEN** the right panel restores to a reasonable size proportional to available space
