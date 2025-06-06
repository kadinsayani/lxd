test_warnings() {
    # Delete previous warnings
    lxc warning delete --all

    # Create a global warning (no node and no project)
    lxc query --wait -X POST -d '{\"type_code\": 0, \"message\": \"global warning\"}' /internal/testing/warnings

    # More valid queries
    lxc query --wait -X POST -d '{\"type_code\": 0, \"message\": \"global warning\", \"project\": \"default\"}' /internal/testing/warnings

    # Update the last warning. This will not create a new warning.
    lxc query --wait -X POST -d '{\"type_code\": 0, \"message\": \"global warning 2\", \"project\": \"default\"}' /internal/testing/warnings

    # There should be two warnings now.
    [ "$(lxc query --wait /1.0/warnings | jq 'length')" = "2" ]

    [ "$(lxc query --wait /1.0/warnings\?recursion=1 | jq 'length')" = "2" ]

    # Invalid query (unknown project)
    ! lxc query --wait -X POST -d '{\"type_code\": 0, \"message\": \"global warning\", \"project\": \"foo\"}' /internal/testing/warnings || false

    # Invalid query (unknown type code)
    ! lxc query --wait -X POST -d '{\"type_code\": 999, \"message\": \"global warning\"}' /internal/testing/warnings || false

    # Both entity type code as entity ID need to be valid otherwise no warning will be created. Note that empty/null values are valid as well.
    ! lxc query --wait -X POST -d '{\"type_code\": 0, \"message\": \"global warning\", \"entity_type\": \"invalid_entity_type\", \"entity_id\": 0}' /internal/testing/warnings || false

    ensure_import_testimage

    # Get image ID from database instead of assuming it
    image_id=$(lxd sql global 'select image_id from images_aliases where name="testimage"' | grep -Eo '[[:digit:]]+')

    # Create a warning with entity type "image" and entity ID ${image_id} (the imported testimage)
    lxc query --wait -X POST -d "{\\\"type_code\\\": 0, \\\"message\\\": \\\"global warning\\\", \\\"entity_type\\\": \\\"image\\\", \\\"entity_id\\\": ${image_id}}" /internal/testing/warnings

    # There should be three warnings now.
    [ "$(lxc warning list --format json | jq 'length')" = "3" ]

    # Test filtering
    [ "$(curl -G --unix-socket "$LXD_DIR/unix.socket" "lxd/1.0/warnings" --data-urlencode "recursion=0" --data-urlencode "filter=status eq new" | jq ".metadata | length")" = "3" ]

    [ "$(curl -G --unix-socket "$LXD_DIR/unix.socket" "lxd/1.0/warnings" --data-urlencode "recursion=0" --data-urlencode "filter=status eq resolved" | jq ".metadata | length")" = "0" ]

    # Acknowledge a warning
    uuid=$(lxc warning list --format json | jq -r '.[] | select(.last_message=="global warning 2") | .uuid')
    lxc warning ack "${uuid}"

    # This should hide the acknowledged
    [ "$(lxc warning list --format json | jq 'length')" = "2" ]

    # ... unless one uses --all.
    [ "$(lxc warning list --all --format json | jq 'length')" = "3" ]

    lxc warning show "${uuid}" | grep "global warning 2"

    # Delete warning
    lxc warning rm "${uuid}"
    ! lxc warning list | grep -F "${uuid}" || false
    ! lxc warning show "${uuid}" || false

    # Delete all warnings
    lxc warning delete --all
    [ -z "$(lxc warning ls --format csv)" ]
}
