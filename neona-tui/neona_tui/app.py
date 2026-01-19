"""Main Textual application for Neona TUI."""

from textual.app import App, ComposeResult
from textual.containers import Container, Vertical
from textual.widgets import Header, Footer, Static, Input, DataTable
from textual.binding import Binding
from textual import on
from rich.text import Text

from .api_client import NeonaClient, NeonaAPIError


class StatusBar(Static):
    """Custom status bar showing daemon status."""
    
    def __init__(self) -> None:
        super().__init__("")
        self.daemon_online = False
    
    def update_status(self, online: bool, task_count: int = 0) -> None:
        """Update the status bar display."""
        self.daemon_online = online
        
        if online:
            status_text = Text()
            status_text.append("● DAEMON ", style="bold green")
            status_text.append(f"| {task_count} tasks", style="cyan")
            self.update(status_text)
        else:
            self.update(Text("○ DAEMON OFFLINE", style="bold red"))


class NeonaTUI(App):
    """Neona Terminal UI - Python Edition with Rich/Textual."""
    
    CSS = """
    StatusBar {
        dock: top;
        height: 1;
        background: $panel;
        color: $text;
        padding: 0 1;
    }
    
    DataTable {
        height: 1fr;
    }
    
    Input {
        dock: bottom;
        border: round $primary;
    }
    
    #message-box {
        dock: bottom;
        height: 1;
        background: $panel;
        color: $success;
        padding: 0 1;
    }
    """
    
    TITLE = "🚀 NEONA Control Plane"
    SUB_TITLE = "Python Edition · Powered by Textual"
    
    BINDINGS = [
        Binding("q", "quit", "Quit", priority=True),
        Binding("r", "refresh", "Refresh"),
        Binding("ctrl+c", "quit", "Quit", show=False),
    ]
    
    def __init__(self) -> None:
        super().__init__()
        self.client = NeonaClient()
        self.tasks: list[dict] = []
        self.message = ""
    
    def compose(self) -> ComposeResult:
        """Create child widgets for the app."""
        yield Header()
        yield StatusBar()
        
        # Main content area
        yield Container(
            DataTable(id="tasks-table"),
            Static(id="message-box"),
            Input(placeholder="Type: add <title> | claim | release | refresh", id="command-input"),
        )
        
        yield Footer()
    
    async def on_mount(self) -> None:
        """Called when app starts."""
        # Setup tasks table
        table = self.query_one("#tasks-table", DataTable)
        table.cursor_type = "row"
        table.add_columns("Status", "ID", "Title")
        
        # Initial data load
        await self.refresh_tasks()
    
    async def refresh_tasks(self) -> None:
        """Fetch and display tasks from daemon."""
        status_bar = self.query_one(StatusBar)
        table = self.query_one("#tasks-table", DataTable)
        
        try:
            # Check daemon health
            online = await self.client.check_health()
            
            if not online:
                status_bar.update_status(False)
                self.show_message("⚠ Daemon offline - start with 'neona daemon'", error=True)
                return
            
            # Fetch tasks
            self.tasks = await self.client.list_tasks()
            status_bar.update_status(True, len(self.tasks))
            
            # Update table
            table.clear()
            for task in self.tasks:
                status = self.format_status(task.get("status", "unknown"))
                task_id = task.get("id", "")[:8]
                title = task.get("task_title", task.get("title", ""))
                table.add_row(status, task_id, title)
            
            self.show_message(f"✓ Loaded {len(self.tasks)} tasks")
            
        except NeonaAPIError as e:
            status_bar.update_status(False)
            self.show_message(f"Error: {e}", error=True)
    
    def format_status(self, status: str) -> Text:
        """Format task status with colors."""
        status_map = {
            "pending": ("○ PENDING", "yellow"),
            "claimed": ("◐ CLAIMED", "blue"),
            "running": ("◑ RUNNING", "magenta"),
            "completed": ("● DONE", "green"),
            "failed": ("✗ FAILED", "red"),
        }
        
        text, color = status_map.get(status.lower(), (status, "white"))
        return Text(text, style=f"bold {color}")
    
    def show_message(self, msg: str, error: bool = False) -> None:
        """Display a message in the message box."""
        self.message = msg
        msg_box = self.query_one("#message-box", Static)
        style = "bold red" if error else "bold green"
        msg_box.update(Text(msg, style=style))
    
    @on(Input.Submitted)
    async def handle_command(self, event: Input.Submitted) -> None:
        """Handle command input."""
        cmd = event.value.strip()
        event.input.value = ""
        
        if not cmd:
            return
        
        parts = cmd.split()
        action = parts[0].lower()
        args = parts[1:]
        
        try:
            if action == "add" and args:
                title = " ".join(args)
                task_id = await self.client.create_task(title)
                self.show_message(f"✓ Created task: {task_id[:8]}")
                await self.refresh_tasks()
            
            elif action == "refresh" or action == "r":
                await self.refresh_tasks()
            
            elif action == "claim":
                table = self.query_one("#tasks-table", DataTable)
                if table.cursor_row is not None and self.tasks:
                    task = self.tasks[table.cursor_row]
                    await self.client.claim_task(task["id"])
                    self.show_message("✓ Task claimed")
                    await self.refresh_tasks()
                else:
                    self.show_message("⚠ No task selected", error=True)
            
            elif action == "release":
                table = self.query_one("#tasks-table", DataTable)
                if table.cursor_row is not None and self.tasks:
                    task = self.tasks[table.cursor_row]
                    await self.client.release_task(task["id"])
                    self.show_message("✓ Task released")
                    await self.refresh_tasks()
                else:
                    self.show_message("⚠ No task selected", error=True)
            
            else:
                self.show_message(f"Unknown: {action} (try: add, claim, release, refresh)", error=True)
                
        except NeonaAPIError as e:
            self.show_message(f"Error: {e}", error=True)
    
    async def action_refresh(self) -> None:
        """Refresh tasks (bound to 'r' key)."""
        await self.refresh_tasks()
    
    async def on_unmount(self) -> None:
        """Called when app closes."""
        await self.client.close()


def main() -> None:
    """Entry point for neona-tui command."""
    app = NeonaTUI()
    app.run()


if __name__ == "__main__":
    main()
