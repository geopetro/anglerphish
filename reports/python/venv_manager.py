#!/usr/bin/env python
"""
Virtual Environment Manager for Anglerphish
Cross-platform venv creation and management for Windows, macOS, and Linux (including PEP 668 systems)
"""

import os
import sys
import subprocess
import json
import platform

# Get the directory where this script is located (reports/python/)
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# Default venv directory - can be overridden via command line or environment variable
# When run from Go service, ANGLERPHISH_VENV_DIR env var will point to persistent location
VENV_DIR = os.environ.get("ANGLERPHISH_VENV_DIR", os.path.join(SCRIPT_DIR, "venv"))


def get_os_info():
    """Get operating system information for platform-specific handling."""
    os_type = platform.system().lower()
    os_release = platform.release()
    os_version = platform.version()
    
    # Detect specific Linux distributions
    distro = None
    distro_version = None
    
    if os_type == "linux":
        try:
            # Try to read /etc/os-release for distribution info
            if os.path.exists("/etc/os-release"):
                with open("/etc/os-release") as f:
                    lines = f.readlines()
                    for line in lines:
                        if line.startswith("ID="):
                            distro = line.strip().split("=")[1].strip('"')
                        elif line.startswith("VERSION_ID="):
                            distro_version = line.strip().split("=")[1].strip('"')
        except:
            pass
    
    return {
        "os_type": os_type,
        "os_release": os_release,
        "os_version": os_version,
        "distro": distro,
        "distro_version": distro_version,
        "is_windows": os_type == "windows",
        "is_macos": os_type == "darwin",
        "is_linux": os_type == "linux"
    }


def get_venv_paths():
    """
    Get platform-specific paths for virtual environment executables.
    
    Returns:
        dict: Dictionary with venv_dir, python, and pip paths
    """
    os_info = get_os_info()
    
    if os_info["is_windows"]:
        python_path = os.path.join(VENV_DIR, "Scripts", "python.exe")
        pip_path = os.path.join(VENV_DIR, "Scripts", "pip.exe")
    else:
        python_path = os.path.join(VENV_DIR, "bin", "python")
        pip_path = os.path.join(VENV_DIR, "bin", "pip")
    
    return {
        "venv_dir": VENV_DIR,
        "python": python_path,
        "pip": pip_path
    }


def venv_exists():
    """
    Check if virtual environment exists and is valid.
    
    Returns:
        bool: True if venv exists with valid python and pip
    """
    paths = get_venv_paths()
    return os.path.isfile(paths["python"]) and os.path.isfile(paths["pip"])


def create_venv():
    """
    Create a new virtual environment.
    Handles platform-specific issues like missing python3-venv on Debian/Ubuntu.
    
    Returns:
        dict: Result with success status and any error messages
    """
    os_info = get_os_info()
    
    # Remove existing broken venv if present
    if os.path.exists(VENV_DIR):
        import shutil
        try:
            shutil.rmtree(VENV_DIR)
        except Exception as e:
            return {
                "success": False,
                "error": f"Could not remove existing venv directory: {e}",
                "advice": "Try manually deleting the venv directory"
            }
    
    # Try to create venv using the venv module
    try:
        import venv
        venv.create(VENV_DIR, with_pip=True, clear=True)
        
        # Verify it was created successfully
        if venv_exists():
            return {"success": True, "message": "Virtual environment created successfully"}
        else:
            raise Exception("Venv created but executables not found")
            
    except Exception as e:
        error_str = str(e).lower()
        
        # Handle missing ensurepip (common on Debian/Ubuntu)
        if "ensurepip" in error_str or "pip" in error_str:
            advice = get_platform_advice(os_info, "missing_pip")
            return {
                "success": False,
                "error": f"Failed to create venv with pip: {e}",
                "advice": advice
            }
        
        # Try subprocess fallback
        try:
            subprocess.check_call(
                [sys.executable, "-m", "venv", VENV_DIR],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
            
            if venv_exists():
                return {"success": True, "message": "Virtual environment created using subprocess"}
            else:
                raise Exception("Venv created but executables not found")
                
        except subprocess.CalledProcessError as e2:
            advice = get_platform_advice(os_info, "venv_creation_failed")
            return {
                "success": False,
                "error": f"Failed to create virtual environment: {e2}",
                "advice": advice
            }
        except Exception as e2:
            return {
                "success": False,
                "error": f"Failed to create virtual environment: {e2}",
                "advice": get_platform_advice(os_info, "venv_creation_failed")
            }


def get_platform_advice(os_info, issue_type):
    """
    Get platform-specific advice for common issues.
    
    Args:
        os_info: OS information dictionary
        issue_type: Type of issue (missing_pip, venv_creation_failed, etc.)
    
    Returns:
        str: Platform-specific advice
    """
    if issue_type == "missing_pip" or issue_type == "venv_creation_failed":
        if os_info["is_linux"]:
            distro = os_info.get("distro", "").lower()
            
            if distro in ["debian", "ubuntu", "linuxmint", "pop"]:
                return "Install python3-venv: sudo apt update && sudo apt install python3-venv python3-pip"
            elif distro in ["centos", "rhel", "fedora", "rocky", "almalinux"]:
                return "Install Python venv: sudo dnf install python3-pip python3-virtualenv"
            elif distro in ["arch", "manjaro"]:
                return "Install Python: sudo pacman -S python python-pip"
            else:
                return "Install python3-venv and python3-pip using your package manager"
        
        elif os_info["is_macos"]:
            return "Ensure Python 3 is installed: brew install python3"
        
        elif os_info["is_windows"]:
            return "Ensure Python 3 is properly installed from python.org with pip included"
    
    return "Please ensure Python 3.7+ is installed with pip support"


def upgrade_pip():
    """
    Upgrade pip in the virtual environment to avoid compatibility issues.
    
    Returns:
        dict: Result with success status
    """
    if not venv_exists():
        return {"success": False, "error": "Virtual environment does not exist"}
    
    paths = get_venv_paths()
    
    try:
        subprocess.check_call(
            [paths["python"], "-m", "pip", "install", "--upgrade", "pip"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
        return {"success": True, "message": "Pip upgraded successfully"}
    except subprocess.CalledProcessError as e:
        # Non-fatal - pip might already be up to date or upgrade might fail
        return {"success": True, "warning": f"Could not upgrade pip: {e}"}


def ensure_venv():
    """
    Ensure virtual environment exists and is ready for use.
    Creates it if necessary and upgrades pip.
    
    Returns:
        dict: Result with success status and paths
    """
    if venv_exists():
        paths = get_venv_paths()
        return {
            "success": True,
            "message": "Virtual environment ready",
            "paths": paths,
            "created": False
        }
    
    # Create the venv
    result = create_venv()
    if not result["success"]:
        return result
    
    # Upgrade pip
    upgrade_result = upgrade_pip()
    
    paths = get_venv_paths()
    return {
        "success": True,
        "message": "Virtual environment created and ready",
        "paths": paths,
        "created": True,
        "pip_upgrade": upgrade_result.get("warning", "Pip upgraded")
    }


def get_python_executable():
    """
    Get the Python executable to use (venv if available).
    
    Returns:
        str: Path to Python executable
    """
    if venv_exists():
        return get_venv_paths()["python"]
    return sys.executable


def get_pip_executable():
    """
    Get the pip executable to use (venv if available).
    
    Returns:
        str or None: Path to pip executable, or None to use python -m pip
    """
    if venv_exists():
        return get_venv_paths()["pip"]
    return None


def get_status():
    """
    Get comprehensive status of the virtual environment.
    
    Returns:
        dict: Complete status information
    """
    os_info = get_os_info()
    paths = get_venv_paths()
    exists = venv_exists()
    
    status = {
        "venv_exists": exists,
        "venv_dir": paths["venv_dir"],
        "venv_python": paths["python"],
        "venv_pip": paths["pip"],
        "system_python": sys.executable,
        "python_version": sys.version,
        "os_info": os_info
    }
    
    if exists:
        # Get venv Python version
        try:
            result = subprocess.run(
                [paths["python"], "--version"],
                capture_output=True,
                text=True
            )
            status["venv_python_version"] = result.stdout.strip() or result.stderr.strip()
        except:
            status["venv_python_version"] = "Unknown"
    
    return status


if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="Manage Python virtual environment for Anglerphish")
    parser.add_argument("--create", action="store_true", help="Create virtual environment")
    parser.add_argument("--ensure", action="store_true", help="Ensure venv exists (create if needed)")
    parser.add_argument("--status", action="store_true", help="Show venv status")
    parser.add_argument("--paths", action="store_true", help="Show venv paths")
    
    args = parser.parse_args()
    
    if args.create:
        result = create_venv()
        print(json.dumps(result, indent=2))
    elif args.ensure:
        result = ensure_venv()
        print(json.dumps(result, indent=2))
    elif args.paths:
        paths = get_venv_paths()
        paths["exists"] = venv_exists()
        print(json.dumps(paths, indent=2))
    elif args.status:
        status = get_status()
        print(json.dumps(status, indent=2))
    else:
        # Default: ensure venv exists
        result = ensure_venv()
        print(json.dumps(result, indent=2))
