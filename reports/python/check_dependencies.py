#!/usr/bin/env python
"""
Check Python dependencies for Anglerphish report generation.
Checks dependencies in the virtual environment if it exists.
"""

import json
import sys
import subprocess
import os

# Import venv_manager for venv handling
try:
    from venv_manager import venv_exists, get_venv_paths, get_os_info
except ImportError:
    # Handle case where script is run directly
    script_dir = os.path.dirname(os.path.abspath(__file__))
    sys.path.insert(0, script_dir)
    from venv_manager import venv_exists, get_venv_paths, get_os_info

# Required packages for report generation
REQUIRED_PACKAGES = [
    {"name": "python-docx", "import_name": "docx", "required": True},
    {"name": "openpyxl", "import_name": "openpyxl", "required": True},
    {"name": "requests", "import_name": "requests", "required": True},
    {"name": "traceback2", "import_name": "traceback2", "required": False},
    {"name": "user-agents", "import_name": "user_agents", "required": False},
    {"name": "geoip2", "import_name": "geoip2", "required": False}
]


def check_package_in_venv(venv_python, import_name):
    """
    Check if a package is installed in the virtual environment.
    
    Args:
        venv_python: Path to venv Python executable
        import_name: Import name of the package to check
    
    Returns:
        tuple: (installed: bool, version: str or None, error: str or None)
    """
    # Create a small Python script to check the import
    check_script = f'''
import sys
try:
    import {import_name}
    version = getattr({import_name}, "__version__", "Unknown")
    print(f"OK|{{version}}")
except ImportError as e:
    print(f"ERROR|{{str(e)}}")
'''
    
    try:
        result = subprocess.run(
            [venv_python, "-c", check_script],
            capture_output=True,
            text=True,
            timeout=10
        )
        
        output = result.stdout.strip()
        if output.startswith("OK|"):
            version = output.split("|", 1)[1]
            return True, version, None
        elif output.startswith("ERROR|"):
            error = output.split("|", 1)[1]
            return False, None, error
        else:
            return False, None, result.stderr or "Unknown error"
            
    except subprocess.TimeoutExpired:
        return False, None, "Check timed out"
    except Exception as e:
        return False, None, str(e)


def check_dependencies():
    """
    Check if all required dependencies are installed.
    Checks in the virtual environment if it exists.
    
    Returns:
        dict: Dictionary with dependency status information
    """
    os_info = get_os_info()
    
    result = {
        "all_installed": True,
        "required_installed": True,
        "python_installed": True,
        "python_version": sys.version.split()[0],
        "venv_exists": False,
        "venv_used": False,
        "dependencies": [],
        "os_info": os_info
    }
    
    # Check if venv exists
    if venv_exists():
        result["venv_exists"] = True
        result["venv_used"] = True
        paths = get_venv_paths()
        venv_python = paths["python"]
        result["venv_python"] = venv_python
        
        # Get venv Python version
        try:
            version_result = subprocess.run(
                [venv_python, "-c", "import sys; print(sys.version.split()[0])"],
                capture_output=True,
                text=True
            )
            result["venv_python_version"] = version_result.stdout.strip()
        except:
            result["venv_python_version"] = "Unknown"
    else:
        # No venv - dependencies won't be available
        result["venv_exists"] = False
        result["venv_used"] = False
        result["venv_message"] = "Virtual environment not found. Click 'Install Dependencies' to create it."
        venv_python = None
    
    # Check each package
    for package in REQUIRED_PACKAGES:
        package_status = {
            "name": package["name"],
            "installed": False,
            "required": package["required"],
            "version": None,
            "error": None
        }
        
        if venv_python:
            # Check in venv
            installed, version, error = check_package_in_venv(venv_python, package["import_name"])
            package_status["installed"] = installed
            package_status["version"] = version
            package_status["error"] = error
        else:
            # No venv - mark as not installed
            package_status["installed"] = False
            package_status["error"] = "Virtual environment not created"
        
        # Update overall status
        if not package_status["installed"]:
            result["all_installed"] = False
            if package["required"]:
                result["required_installed"] = False
        
        result["dependencies"].append(package_status)
    
    return result


if __name__ == "__main__":
    # Run the check and print results as JSON
    results = check_dependencies()
    print(json.dumps(results, indent=2))
