var groups = []

// Save attempts to POST or PUT to /groups/
function save(id) {
    var targets = []
    
    // Get targets from the table
    $.each($("#targetsTable").DataTable().rows().data(), function (i, target) {

        const email = unescapeHtml(target[2]).trim();
        const phone = unescapeHtml(target[3]).trim();
    
        // Skip if both are empty
        if (!email && !phone) return;

        targets.push({
            first_name: unescapeHtml(target[0]),
            last_name: unescapeHtml(target[1]),
            email: unescapeHtml(target[2]).trim().toLowerCase(),
            phone: unescapeHtml(target[3]).replace(/\D/g, ''),
            position: unescapeHtml(target[4]),
            custom: unescapeHtml(target[5])
        })
    })
    
    // Get original targets from the hidden field if it exists
    // var originalTargetsJson = $("#original_targets").val();
    // if (originalTargetsJson && id != -1) {
    //     try {
    //         var originalTargets = JSON.parse(originalTargetsJson);
    //         console.log("Original targets:", originalTargets);
            
    //         // Merge targets, avoiding duplicates
    //         var mergedTargets = [...targets];
    //         var emailMap = {};
    //         var phoneMap = {};
            
    //         // Create maps of existing targets by email and phone
    //         targets.forEach(function(target) {
    //             if (target.email) {
    //                 const normalizedEmail = target.email.trim().toLowerCase();
    //                 emailMap[normalizedEmail] = true;
    //             }
    //             if (target.phone) {
    //                 const normalizedPhone = target.phone.replace(/\D/g, '');
    //                 phoneMap[normalizedPhone] = true;
    //             }
    //         });
            
    //         // Add original targets that aren't already in the table
    //         originalTargets.forEach(function(target) {
    //             var isDuplicate = false;
                
    //             // Check if this target is already in our list by email or phone
    //             if (target.email) {
    //                 const normalizedEmail = target.email.trim().toLowerCase();
    //                 if (emailMap[normalizedEmail]) {
    //                     isDuplicate = true;
    //                 }
    //             }
    //             if (target.phone) {
    //                 const normalizedPhone = target.phone.replace(/\D/g, '');
    //                 if (phoneMap[normalizedPhone]) {
    //                     isDuplicate = true;
    //                 }
    //             }
                
    //             // If not a duplicate, add it to the merged list
    //             if (!isDuplicate) {
    //                 mergedTargets.push(target);
    //             }
    //         });
            
    //         targets = mergedTargets;
    //     } catch (e) {
    //         console.error("Error parsing original targets:", e);
    //     }
    // }
    
    // Log the targets being sent to the server for debugging
    // console.log("Saving group with targets:", targets);
    
    var group = {
        name: $("#name").val(),
        targets: targets
    }
    // Submit the group
    if (id != -1) {
        // If we're just editing an existing group,
        // we need to PUT /groups/:id
        group.id = id
        api.groupId.put(group)
            .success(function (data) {
                successFlash("Group updated successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                // console.error("Error updating group:", data);
                modalError(data.responseJSON ? data.responseJSON.message : "An error occurred while updating the group")
            })
    } else {
        // Else, if this is a new group, POST it
        // to /groups
        api.groups.post(group)
            .success(function (data) {
                successFlash("Group added successfully!")
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                // console.error("Error saving group:", data);
                modalError(data.responseJSON ? data.responseJSON.message : "An error occurred while saving the group")
            })
    }
}

function dismiss() {
    $("#targetsTable").dataTable().DataTable().clear().draw()
    $("#name").val("")
    $("#modal\\.flashes").empty()
}

function edit(id) {
    targets = $("#targetsTable").dataTable({
        destroy: true, // Destroy any other instantiated table - http://datatables.net/manual/tech-notes/3#destroy
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }]
    })
    $("#modalSubmit").unbind('click').click(function () {
        save(id)
    })
    if (id == -1) {
        $("#groupModalLabel").text("New Group");
        // Don't try to fetch a group when creating a new one
        // Clear the table just in case
        targets.DataTable().clear().draw();
    } else {
        $("#groupModalLabel").text("Edit Group");
        api.groupId.get(id)
            .success(function (group) {
                // console.log("Group data received:", group);
                $("#name").val(group.name)
                targetRows = []
                if (group.targets && group.targets.length > 0) {
                    // console.log("Group targets:", group.targets);
                    $.each(group.targets, function (i, record) {
                      targetRows.push([
                          escapeHtml(record.first_name),
                          escapeHtml(record.last_name),
                          escapeHtml(record.email),
                          escapeHtml(record.phone),
                          escapeHtml(record.position),
                          escapeHtml(record.custom),
                          '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
                      ])
                    });
                } else {
                    // console.warn("No targets found in group or targets array is empty");
                }
                targets.DataTable().rows.add(targetRows).draw()
                
                // Store the original targets in a hidden field so we can preserve them when saving
                $("#original_targets").val(JSON.stringify(group.targets));
            })
            .error(function () {
                errorFlash("Error fetching group")
            })
    }
    // Handle file uploads
    $("#csvupload").fileupload({
        url: "/api/import/group",
        dataType: "json",
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        },
        add: function (e, data) {
            $("#modal\\.flashes").empty()
            var acceptFileTypes = /(csv|txt)$/i;
            var filename = data.originalFiles[0]['name']
            if (filename && !acceptFileTypes.test(filename.split(".").pop())) {
                modalError("Unsupported file extension (use .csv or .txt)")
                return false;
            }
            data.submit();
        },
        done: function (e, data) {
            $.each(data.result, function (i, record) {
                addTarget(
                    record.first_name,
                    record.last_name,
                    record.email,
                    record.phone,
                    record.position,
                    record.custom);
            });
            targets.DataTable().draw();
        }
    })
}

var downloadCSVTemplate = function () {
    var csvScope = [{
        'First_Name': 'Example',
        'Last_Name': 'User',
        'Email': 'foobar@example.com',
        'Phone': '',
        'Position': 'Systems Administrator',
        'Custom': 'Custom value'
    }, {
        'First_Name': 'Example2',
        'Last_Name': 'User2',
        'Email': '',
        'Phone': '+1234567890',
        'Position': 'Human Resources',
        'Custom': 'Foo bar'
    }]
    var filename = 'group_template.csv'
    var csvString = Papa.unparse(csvScope, {})
    var csvData = new Blob([csvString], {
        type: 'text/csv;charset=utf-8;'
    });
    if (navigator.msSaveBlob) {
        navigator.msSaveBlob(csvData, filename);
    } else {
        var csvURL = window.URL.createObjectURL(csvData);
        var dlLink = document.createElement('a');
        dlLink.href = csvURL;
        dlLink.setAttribute('download', filename)
        document.body.appendChild(dlLink)
        dlLink.click();
        document.body.removeChild(dlLink)
    }
}

var downloadGroup = function(id) {
    // Get the group details
    api.groupId.get(id)
        .success(function(group) {
            // Create CSV content with underscores in headers for easy re-upload
            var csvContent = "First_Name,Last_Name,Email,Phone,Position,Custom\n";
            
            // Add each target to the CSV
            $.each(group.targets, function(i, target) {
                // Properly escape fields that might contain commas
                var firstName = escapeCsvField(target.first_name);
                var lastName = escapeCsvField(target.last_name);
                var email = escapeCsvField(target.email);
                var phone = escapeCsvField(target.phone);
                var position = escapeCsvField(target.position);
                var custom = escapeCsvField(target.custom);
                
                // Add the row to CSV content
                csvContent += firstName + "," + lastName + "," + email + "," + phone + "," + position + "," + custom + "\n";
            });
            
            // Create a blob with the CSV content
            var blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
            
            // Create a safe filename based on the group name
            var filename = "group_" + group.name.replace(/[^\w\-]+/g, '_').toLowerCase() + ".csv";
            
            // Handle different browser download methods
            if (navigator.msSaveBlob) { // IE 10+
                navigator.msSaveBlob(blob, filename);
            } else {
                // For other browsers
                var link = document.createElement("a");
                if (link.download !== undefined) { // Feature detection
                    // Create a URL for the blob
                    var url = URL.createObjectURL(blob);
                    link.setAttribute("href", url);
                    link.setAttribute("download", filename);
                    link.style.visibility = 'hidden';
                    document.body.appendChild(link);
                    link.click();
                    document.body.removeChild(link);
                    URL.revokeObjectURL(url);
                }
            }
            
            successFlash("Group exported successfully!");
        })
        .error(function() {
            errorFlash("Error fetching group data for export");
        });
}

// Helper function to escape CSV fields
function escapeCsvField(field) {
    if (field === null || field === undefined) {
        return '';
    }
    
    // Convert to string
    field = String(field);
    
    // If the field contains commas, quotes, or newlines, wrap it in quotes
    if (field.includes(',') || field.includes('"') || field.includes('\n')) {
        // Double any existing quotes
        field = field.replace(/"/g, '""');
        // Wrap in quotes
        field = '"' + field + '"';
    }
    
    return field;
}

var toggleLock = function (id) {
    api.groupId.lock(id)
        .success(function (group) {
            var locked = group.locked
            var btn = $("button[onclick='toggleLock(" + id + ")']")
            btn.removeClass('btn-default btn-warning')
               .addClass(locked ? 'btn-warning' : 'btn-default')
               .attr('title', locked ? 'Unlock group' : 'Lock group')
               .find('i')
               .removeClass('fa-lock fa-unlock-alt')
               .addClass(locked ? 'fa-lock' : 'fa-unlock-alt')
            var row = btn.closest('tr')
            var nameCell = row.find('td:first')
            var plainName = nameCell.text().trim()
            if (locked) {
                nameCell.html("<i class='fa fa-lock' style='margin-right:5px;color:#e6a817;' title='Locked — not available in campaign pickers'></i>" + escapeHtml(plainName))
            } else {
                nameCell.text(plainName)
            }
            // keep local groups array in sync
            var g = groups.find(function(x) { return x.id === id })
            if (g) g.locked = locked
        })
        .error(function () {
            errorFlash("Error updating group lock status")
        })
}

var deleteGroup = function (id) {
    var group = groups.find(function (x) {
        return x.id === id
    })
    if (!group) {
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the group. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(group.name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.groupId.delete(id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Group Deleted!',
                'This group has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function addTarget(firstNameInput, lastNameInput, emailInput, phoneInput, positionInput, customInput) {
    // Create new data row.
    var email = emailInput ? escapeHtml(emailInput).toLowerCase() : "";
    var phone = phoneInput ? escapeHtml(phoneInput) : "";
    var newRow = [
        escapeHtml(firstNameInput),
        escapeHtml(lastNameInput),
        email,
        phone,
        escapeHtml(positionInput),
        escapeHtml(customInput),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];

    // Check table to see if email or phone already exists.
    var targetsTable = targets.DataTable();
    var existingRowIndex = -1;
    
    // First check if we have a match by both email and phone (if both are provided)
    if (email && phone) {
        // console.log("Checking for target with both email and phone:", email, phone);
        targetsTable.rows().every(function(rowIdx) {
            var rowData = this.data();
            if (rowData[2] === email && rowData[3] === phone) {
                // console.log("Found match by both email and phone at row:", rowIdx);
                existingRowIndex = rowIdx;
                return false; // Break the loop
            }
            return true;
        });
    }
    
    // If no match found by both, check if email exists
    if (existingRowIndex < 0 && email) {
        // console.log("Checking for target with email:", email);
        targetsTable.rows().every(function(rowIdx) {
            var rowData = this.data();
            if (rowData[2] === email) {
                // console.log("Found match by email at row:", rowIdx);
                existingRowIndex = rowIdx;
                return false; // Break the loop
            }
            return true;
        });
    }
    
    // If still no match found and phone is provided, check if phone exists
    if (existingRowIndex < 0 && phone) {
        // console.log("Checking for target with phone:", phone);
        targetsTable.rows().every(function(rowIdx) {
            var rowData = this.data();
            if (rowData[3] === phone) {
                // console.log("Found match by phone at row:", rowIdx);
                existingRowIndex = rowIdx;
                return false; // Break the loop
            }
            return true;
        });
    }
    
    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        targetsTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        targetsTable.row.add(newRow);
    }
}

function load() {
    $("#groupTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.groups.summary()
        .success(function (response) {
            // console.log("Group summary response:", response);
            $("#loading").hide()
            if (response.total > 0) {
                groups = response.groups
                // console.log("Groups data:", groups);
                $("#emptyMessage").hide()
                $("#groupTable").show()
                var groupTable = $("#groupTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                groupTable.clear();
                groupRows = []
                $.each(groups, function (i, group) {
                    var lockIcon = group.locked ? 'fa-lock' : 'fa-unlock-alt'
                    var lockTitle = group.locked ? 'Unlock group' : 'Lock group'
                    var lockBtnClass = group.locked ? 'btn-warning' : 'btn-default'
                    var nameCell = group.locked
                        ? "<i class='fa fa-lock' style='margin-right:5px;color:#e6a817;' title='Locked — not available in campaign pickers'></i>" + escapeHtml(group.name)
                        : escapeHtml(group.name)
                    groupRows.push([
                        nameCell,
                        escapeHtml(group.num_targets),
                        moment(group.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'>\
                    <button class='btn btn-primary' data-toggle='modal' data-backdrop='static' data-target='#modal' onclick='edit(" + group.id + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button>\
                    <button class='btn btn-primary' onclick='downloadGroup(" + group.id + ")'>\
                    <i class='fa fa-download'></i>\
                    </button>\
                    <button class='btn " + lockBtnClass + "' onclick='toggleLock(" + group.id + ")' title='" + lockTitle + "'>\
                    <i class='fa " + lockIcon + "'></i>\
                    </button>\
                    <button class='btn btn-danger' onclick='deleteGroup(" + group.id + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                groupTable.rows.add(groupRows).draw()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching groups")
        })
}

$(document).ready(function () {
    load()
    // Setup the event listeners
    // Handle manual additions
    $("#targetForm").submit(function () {
        // Validate the form data
        var targetForm = document.getElementById("targetForm")
        if (!targetForm.checkValidity()) {
            targetForm.reportValidity()
            return
        }
        
        // Check if either email or phone is provided
        var email = $("#email").val();
        var phone = $("#phone").val();
        if (!email && !phone) {
            modalError("Either Email or Phone must be provided");
            return false;
        }
        
        addTarget(
            $("#firstName").val(),
            $("#lastName").val(),
            email,
            phone,
            $("#position").val(),
            $("#custom").val());
        targets.DataTable().draw();

        // Reset user input.
        $("#targetForm>div>input").val('');
        $("#firstName").focus();
        return false;
    });
    // Handle Deletion
    $("#targetsTable").on("click", "span>i.fa-trash-o", function () {
        targets.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    $("#modal").on("hide.bs.modal", function () {
        dismiss();
    });
    $("#csv-template").click(downloadCSVTemplate)
});
