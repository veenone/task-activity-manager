package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// folderCreatePayload / folderRenamePayload / folderDeletePayload are the JSON
// shapes stored in the corresponding pending rows' values.
type folderCreatePayload struct {
	Name       string `json:"name"`
	ParentPath string `json:"parentPath"`
}

type folderRenameSnapshot struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type folderDeleteSnapshot struct {
	Name       string `json:"name"`
	ParentPath string `json:"parentPath"`
}

// CreateFolder adds a Test Repository folder under parentPath and queues it for
// creation in Jira on commit (FR-13.3). parentPath is "" for a top-level
// folder. The new folder's id is its full path (parentPath + "/" + name).
func (r *Repository) CreateFolder(profileID, parentPath, name string) (Folder, error) {
	if err := validateFolderName(name); err != nil {
		return Folder{}, err
	}
	path := parentPath + "/" + name

	tx, err := r.db.Begin()
	if err != nil {
		return Folder{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if parentPath != "" {
		if err := folderMustExist(tx, profileID, parentPath); err != nil {
			return Folder{}, err
		}
	}
	exists, err := folderExists(tx, profileID, path)
	if err != nil {
		return Folder{}, err
	}
	if exists {
		return Folder{}, fmt.Errorf("a folder named %q already exists here", name)
	}

	if _, err := tx.Exec(
		`INSERT INTO test_folder (profile_id, id, parent_id, name) VALUES (?, ?, ?, ?)`,
		profileID, path, parentPath, name,
	); err != nil {
		return Folder{}, fmt.Errorf("insert folder: %w", err)
	}

	payload, _ := json.Marshal(folderCreatePayload{Name: name, ParentPath: parentPath})
	if err := upsertPendingChange(
		tx, profileID, entityFolderCreate, path, "folder", "", string(payload), "",
	); err != nil {
		return Folder{}, err
	}
	if err := writeAudit(
		tx, profileID, entityFolderCreate, path, "create-folder-local", "folder", "", name, "",
	); err != nil {
		return Folder{}, err
	}
	if err := tx.Commit(); err != nil {
		return Folder{}, fmt.Errorf("commit create folder: %w", err)
	}
	return Folder{ID: path, ParentID: parentPath, Name: name}, nil
}

// RenameFolder renames a folder, cascading the path change to descendant
// folders and the Tests filed under them (FR-13.3). Queued for commit.
func (r *Repository) RenameFolder(profileID, path, newName string) error {
	if err := validateFolderName(newName); err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var oldName, parentPath string
	err = tx.QueryRow(
		`SELECT name, parent_id FROM test_folder WHERE profile_id = ? AND id = ?`,
		profileID, path,
	).Scan(&oldName, &parentPath)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("folder %q not found", path)
	}
	if err != nil {
		return fmt.Errorf("read folder: %w", err)
	}
	newPath := parentPath + "/" + newName
	if newPath == path {
		return nil
	}
	exists, err := folderExists(tx, profileID, newPath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("a folder named %q already exists here", newName)
	}

	if err := renameFolderTree(tx, profileID, path, newPath, newName); err != nil {
		return err
	}

	before, _ := json.Marshal(folderRenameSnapshot{Path: path, Name: oldName})
	after, _ := json.Marshal(folderRenameSnapshot{Path: newPath, Name: newName})
	if err := upsertPendingChange(
		tx, profileID, entityFolderRename, newPath, "folder", string(before), string(after), "",
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityFolderRename, newPath, "rename-folder-local", "folder", path, newPath, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteFolder removes an empty folder (no subfolders, no Tests) and queues the
// deletion for commit (FR-13.3).
func (r *Repository) DeleteFolder(profileID, path string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var name, parentPath string
	err = tx.QueryRow(
		`SELECT name, parent_id FROM test_folder WHERE profile_id = ? AND id = ?`,
		profileID, path,
	).Scan(&name, &parentPath)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("folder %q not found", path)
	}
	if err != nil {
		return fmt.Errorf("read folder: %w", err)
	}

	var childCount int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM test_folder WHERE profile_id = ? AND parent_id = ?`,
		profileID, path,
	).Scan(&childCount); err != nil {
		return fmt.Errorf("count subfolders: %w", err)
	}
	if childCount > 0 {
		return fmt.Errorf("folder is not empty — it has subfolders")
	}
	var testCount int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM test_case WHERE profile_id = ? AND folder_id = ?`,
		profileID, path,
	).Scan(&testCount); err != nil {
		return fmt.Errorf("count tests: %w", err)
	}
	if testCount > 0 {
		return fmt.Errorf("folder is not empty — it contains %d tests", testCount)
	}

	if _, err := tx.Exec(
		`DELETE FROM test_folder WHERE profile_id = ? AND id = ?`, profileID, path,
	); err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}

	snapshot, _ := json.Marshal(folderDeleteSnapshot{Name: name, ParentPath: parentPath})
	if err := upsertPendingChange(
		tx, profileID, entityFolderDelete, path, "folder", string(snapshot), "1", "",
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityFolderDelete, path, "delete-folder-local", "folder", string(snapshot), "", "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// renameFolderTree rewrites a folder's path plus its descendants' paths and the
// folder_id of any Tests beneath it.
func renameFolderTree(tx *sql.Tx, profileID, oldPath, newPath, newName string) error {
	// The folder itself: new id + name.
	if _, err := tx.Exec(
		`UPDATE test_folder SET id = ?, name = ? WHERE profile_id = ? AND id = ?`,
		newPath, newName, profileID, oldPath,
	); err != nil {
		return fmt.Errorf("rename folder: %w", err)
	}
	// Descendants: rewrite the oldPath prefix in id and parent_id.
	if _, err := tx.Exec(
		`UPDATE test_folder
		   SET id = ? || substr(id, ?),
		       parent_id = ? || substr(parent_id, ?)
		 WHERE profile_id = ? AND id LIKE ?`,
		newPath, len(oldPath)+1, newPath, len(oldPath)+1, profileID, oldPath+"/%",
	); err != nil {
		return fmt.Errorf("rename descendants: %w", err)
	}
	// Tests filed in the folder or its descendants.
	if _, err := tx.Exec(
		`UPDATE test_case SET folder_id = ? || substr(folder_id, ?)
		 WHERE profile_id = ? AND (folder_id = ? OR folder_id LIKE ?)`,
		newPath, len(oldPath)+1, profileID, oldPath, oldPath+"/%",
	); err != nil {
		return fmt.Errorf("re-file tests: %w", err)
	}
	return nil
}

func validateFolderName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a folder name is required")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("a folder name cannot contain '/'")
	}
	return nil
}

func folderExists(tx *sql.Tx, profileID, path string) (bool, error) {
	var one int
	err := tx.QueryRow(
		`SELECT 1 FROM test_folder WHERE profile_id = ? AND id = ?`, profileID, path,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check folder: %w", err)
	}
	return true, nil
}

func folderMustExist(tx *sql.Tx, profileID, path string) error {
	ok, err := folderExists(tx, profileID, path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("parent folder %q not found", path)
	}
	return nil
}
