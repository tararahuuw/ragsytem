package cache

import "fmt"

// namespace prefixes every key so ragsystem entries are easy to spot/flush and
// never collide with other apps sharing a Redis instance.
const namespace = "ragsystem:"

// --- Organization keys ---

// KeyOrgExists caches the ExistsActive(code) validation used at register time.
func KeyOrgExists(code string) string { return namespace + "org:exists:" + code }

// KeyOrgGet caches a single organization by code.
func KeyOrgGet(code string) string { return namespace + "org:get:" + code }

// KeyOrgList caches the full organization list.
func KeyOrgList() string { return namespace + "org:list" }

// OrgKeys returns every cache key affected by a write to one org code — the set
// to invalidate on create/update/delete.
func OrgKeys(code string) []string {
	return []string{KeyOrgExists(code), KeyOrgGet(code), KeyOrgList()}
}

// --- Document keys ---

// docListAll is the sentinel scope for the admin "all organizations" list, so it
// gets its own stable key (KeyDocList("")).
const docListAll = "__all__"

// KeyDocList caches the document list for a scope. Empty scope = admin/all-orgs.
// The scope (organizationCode) is part of the key so tenants never share a list.
func KeyDocList(scope string) string {
	if scope == "" {
		scope = docListAll
	}
	return namespace + "doc:list:" + scope
}

// KeyDocByID caches one document's metadata by id.
func KeyDocByID(id uint) string { return fmt.Sprintf("%sdoc:id:%d", namespace, id) }

// DocListKeysForOrg returns the list keys to invalidate when a new document lands
// in orgCode: that org's list plus the admin all-orgs list.
func DocListKeysForOrg(orgCode string) []string {
	return []string{KeyDocList(orgCode), KeyDocList("")}
}
