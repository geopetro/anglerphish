#!/usr/bin/env python
"""
Install Python dependencies for Anglerphish report generation.
Uses virtual environment to avoid PEP 668 "externally-managed-environment" issues.
"""

import json
import subprocess
import sys
import os

# Import venv_manager for venv handling
try:
    from venv_manager import ensure_venv, get_venv_paths, venv_exists, get_os_info
except ImportError:
    # Handle case where script is run directly
    script_dir = os.path.dirname(os.path.abspath(__file__))
    sys.path.insert(0, script_dir)
    from venv_manager import ensure_venv, get_venv_paths, venv_exists, get_os_info

# Get the directory where this script is located
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REQUIREMENTS_PATH = os.path.join(SCRIPT_DIR, "requirements.txt")


def install_dependencies():
    """
    Install all dependencies from requirements.txt into the virtual environment.
    Creates the venv if it doesn't exist.
    
    Returns:
        dict: Result with success status and details
    """
    result = {
        "success": False,
        "message": "",
        "details": "",
        "os_info": get_os_info()
    }
    
    # Step 1: Check requirements.txt exists
    if not os.path.exists(REQUIREMENTS_PATH):
        result["message"] = "requirements.txt not found"
        result["details"] = f"Expected at: {REQUIREMENTS_PATH}"
        return result
    
    # Step 2: Ensure virtual environment exists
    venv_result = ensure_venv()
    if not venv_result["success"]:
        result["message"] = "Failed to create virtual environment"
        result["details"] = venv_result.get("error", "Unknown error")
        result["advice"] = venv_result.get("advice", "")
        return result
    
    # Step 3: Get venv paths
    paths = get_venv_paths()
    venv_python = paths["python"]
    
    # Step 4: Install dependencies using venv pip
    try:
        # Use venv python to run pip
        pip_cmd = [
            venv_python, "-m", "pip", "install",
            "-r", REQUIREMENTS_PATH,
            "--disable-pip-version-check"  # Reduce noise
        ]
        
        process = subprocess.Popen(
            pip_cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True
        )
        
        stdout, stderr = process.communicate()
        
        if process.returncode == 0:
            result["success"] = True
            result["message"] = "Dependencies installed successfully"
            result["details"] = stdout if stdout else "All packages installed"
            result["venv_path"] = paths["venv_dir"]
            result["python_path"] = venv_python
        else:
            result["message"] = "Failed to install dependencies"
            result["details"] = stderr if stderr else stdout
            
            # Check for common errors
            if "externally-managed-environment" in (stderr or ""):
                result["advice"] = "Unexpected PEP 668 error - venv should have prevented this"
            elif "No module named pip" in (stderr or ""):
                result["advice"] = venv_result.get("advice", "Reinstall Python with pip support")
        
        return result
        
    except FileNotFoundError as e:
        result["message"] = "Python executable not found"
        result["details"] = f"Could not find: {venv_python}"
        result["advice"] = "The virtual environment may be corrupted. Try recreating it."
        return result
        
    except Exception as e:
        result["message"] = f"Error during installation: {str(e)}"
        result["details"] = str(e)
        return result


def get_installation_status():
    """
    Get status of installation environment.
    
    Returns:
        dict: Status information
    """
    paths = get_venv_paths()
    
    return {
        "venv_exists": venv_exists(),
        "venv_dir": paths["venv_dir"],
        "venv_python": paths["python"],
        "requirements_path": REQUIREMENTS_PATH,
        "requirements_exists": os.path.exists(REQUIREMENTS_PATH),
        "os_info": get_os_info()
    }


if __name__ == "__main__":
    # Run installation and output result as JSON
    result = install_dependencies()
    print(json.dumps(result, indent=2))
