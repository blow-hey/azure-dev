#!/usr/bin/env pwsh
param(
    [string] $InstallFolder = ""
)

function isLinuxOrMac {
    return $IsLinux -or $IsMacOS
}

if (!$InstallFolder) {
    $InstallFolder = "$($env:LocalAppData)\Programs\Azure Dev CLI"
    if (isLinuxOrMac) {
        $InstallFolder = "/usr/local/bin"
    }
}

if (isLinuxOrMac) {
    $installLocation = "$InstallFolder/azd"

    if (!(Test-Path $installLocation)) {
        Write-Host "azd is not installed at $installLocation. To install run:"
        Write-Host "pwsh -c `"Invoke-RestMethod 'https://aka.ms/install-azd.ps1' | Invoke-Expression`""
        exit 0
    }

    Write-Host "Removing install: $installLocation"
    test -w $installLocation
    if ($LASTEXITCODE) {
        Write-Host "Writing to $InstallFolder/ requires elevated permission. You may be promtped to enter credentials."
        sudo rm $installLocation
        if ($LASTEXITCODE) {
            Write-Error "Could not remove azd from $installLocation"
            exit 1
        }
    } else {
        Remove-Item $installLocation
    }
} else {
    if (Test-Path $InstallFolder) {
        Write-Host "Remove Install folder: $InstallFolder"
        Remove-Item $InstallFolder -Recurse -Force
    } else {
        Write-Host "azd is not installed at $InstallFolder. To install run:"
        Write-Host "powershell -ex AllSigned -c `"Invoke-RestMethod 'https://aka.ms/install-azd.ps1' | Invoke-Expression`"`n"
    }

    # $env:Path, [Environment]::GetEnvironmentVariable('PATH'), Get-ItemProperty,
    # and setx all expand variables (e.g. %JAVA_HOME%) in the value. Writing the
    # expanded paths back into the environment would be destructive so instead, read
    # the PATH entry directly from the registry with the DoNotExpandEnvironmentNames
    # option and update the PATH entry in the registry.
    try {
        . {
            # Wrap the Microsoft.Win32.Registry calls in a script block to
            # prevent the type intializer from attempting to initialize those
            # objects in non-Windows environments.
            $registryKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
            $originalPath = $registryKey.GetValue(`
                'PATH', `
                '', `
                [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames `
            )
            $originalValueKind = $registryKey.GetValueKind('PATH')
        }
        $pathParts = $originalPath -split ';'

        if ($pathParts -contains $InstallFolder) {
            Write-Host "Removing $InstallFolder from PATH"
            $newPathParts = $pathParts.Where({ $_ -ne $InstallFolder })
            $newPath = $newPathParts -join ';'

            $registryKey.SetValue( `
                'PATH', `
                $newPath, `
                $originalValueKind `
            )

            # Calling this method ensures that a WM_SETTINGCHANGE message is
            # sent to top level windows without having to pinvoke from
            # PowerShell. Setting to $null deletes the variable if it exists.
            [Environment]::SetEnvironmentVariable( `
                'AZD_INSTALLER_NOOP', `
                $null, `
                [EnvironmentVariableTarget]::User `
            )
        } else {
            Write-Host "Could not find an entry for $InstallFolder in PATH"
        }
    } finally {
        if ($registryKey) {
            $registryKey.Close()
        }
    }
}
