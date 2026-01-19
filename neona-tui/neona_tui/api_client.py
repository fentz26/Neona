"""HTTP API client for Neona daemon."""

import httpx
from typing import Any, Optional


class NeonaAPIError(Exception):
    """Raised when API request fails."""
    pass


class NeonaClient:
    """Async HTTP client for Neona daemon API."""
    
    def __init__(self, base_url: str = "http://127.0.0.1:7466"):
        """Initialize client with base URL.
        
        Args:
            base_url: Base URL of Neona daemon (default: http://127.0.0.1:7466)
        """
        self.base_url = base_url
        self.client = httpx.AsyncClient(base_url=base_url, timeout=10.0)
    
    async def check_health(self) -> bool:
        """Check if daemon is reachable.
        
        Returns:
            True if daemon is online, False otherwise
        """
        try:
            response = await self.client.get("/tasks")
            return response.status_code == 200
        except Exception:
            return False
    
    async def list_tasks(self, status_filter: str = "") -> list[dict[str, Any]]:
        """List all tasks, optionally filtered by status.
        
        Args:
            status_filter: Filter by status (pending, claimed, running, completed, failed)
            
        Returns:
            List of task dictionaries
            
        Raises:
            NeonaAPIError: If API request fails
        """
        try:
            params = {"status": status_filter} if status_filter else {}
            response = await self.client.get("/tasks", params=params)
            response.raise_for_status()
            return response.json()
        except httpx.HTTPError as e:
            raise NeonaAPIError(f"Failed to list tasks: {e}")
    
    async def get_task(self, task_id: str) -> dict[str, Any]:
        """Get details for a specific task.
        
        Args:
            task_id: Task ID
            
        Returns:
            Task details dictionary
            
        Raises:
            NeonaAPIError: If API request fails
        """
        try:
            response = await self.client.get(f"/tasks/{task_id}")
            response.raise_for_status()
            return response.json()
        except httpx.HTTPError as e:
            raise NeonaAPIError(f"Failed to get task {task_id}: {e}")
    
    async def create_task(self, title: str, description: str = "") -> str:
        """Create a new task.
        
        Args:
            title: Task title
            description: Task description (optional)
            
        Returns:
            Created task ID
            
        Raises:
            NeonaAPIError: If API request fails
        """
        try:
            payload = {"title": title, "description": description}
            response = await self.client.post("/tasks", json=payload)
            response.raise_for_status()
            data = response.json()
            return data.get("id", "")
        except httpx.HTTPError as e:
            raise NeonaAPIError(f"Failed to create task: {e}")
    
    async def claim_task(self, task_id: str) -> None:
        """Claim a task.
        
        Args:
            task_id: Task ID to claim
            
        Raises:
            NeonaAPIError: If API request fails
        """
        try:
            response = await self.client.post(f"/tasks/{task_id}/claim")
            response.raise_for_status()
        except httpx.HTTPError as e:
            raise NeonaAPIError(f"Failed to claim task {task_id}: {e}")
    
    async def release_task(self, task_id: str) -> None:
        """Release a claimed task.
        
        Args:
            task_id: Task ID to release
            
        Raises:
            NeonaAPIError: If API request fails
        """
        try:
            response = await self.client.post(f"/tasks/{task_id}/release")
            response.raise_for_status()
        except httpx.HTTPError as e:
            raise NeonaAPIError(f"Failed to release task {task_id}: {e}")
    
    async def close(self) -> None:
        """Close the HTTP client."""
        await self.client.aclose()
