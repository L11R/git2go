package git

/*
#include <git2.h>

extern int _go_git_tag_foreach(git_repository *repo, void *payload);
*/
import "C"
import (
	"bytes"
	"runtime"
	"strings"
	"unsafe"
)

// Tag
type Tag struct {
	doNotCompare
	Object
	cast_ptr *C.git_tag
}

func (t *Tag) AsObject() *Object {
	return &t.Object
}

func (t *Tag) Message() string {
	ret := C.GoString(C.git_tag_message(t.cast_ptr))
	runtime.KeepAlive(t)
	return ret
}

func (t *Tag) Name() string {
	ret := C.GoString(C.git_tag_name(t.cast_ptr))
	runtime.KeepAlive(t)
	return ret
}

func (t *Tag) Tagger() *Signature {
	cast_ptr := C.git_tag_tagger(t.cast_ptr)
	ret := newSignatureFromC(cast_ptr)
	runtime.KeepAlive(t)
	return ret
}

func (t *Tag) Target() *Object {
	var ptr *C.git_object
	ret := C.git_tag_target(&ptr, t.cast_ptr)
	runtime.KeepAlive(t)
	if ret != 0 {
		return nil
	}

	return allocObject(ptr, t.repo)
}

func (t *Tag) TargetId() *Oid {
	ret := newOidFromC(C.git_tag_target_id(t.cast_ptr))
	runtime.KeepAlive(t)
	return ret
}

func (t *Tag) TargetType() ObjectType {
	ret := ObjectType(C.git_tag_target_type(t.cast_ptr))
	runtime.KeepAlive(t)
	return ret
}

type TagsCollection struct {
	doNotCompare
	repo *Repository
}

func (c *TagsCollection) Create(name string, obj Objecter, tagger *Signature, message string) (*Oid, error) {

	oid := new(Oid)

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	cmessage := C.CString(message)
	defer C.free(unsafe.Pointer(cmessage))

	taggerSig, err := tagger.toC()
	if err != nil {
		return nil, err
	}
	defer C.git_signature_free(taggerSig)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	o := obj.AsObject()
	ret := C.git_tag_create(oid.toC(), c.repo.ptr, cname, o.ptr, taggerSig, cmessage, 0)
	runtime.KeepAlive(c)
	runtime.KeepAlive(obj)
	if ret < 0 {
		return nil, MakeGitError(ret)
	}

	return oid, nil
}

// CreateTagBuffer creates a tag and write it into a Golang-buffer.
// libgit2 does not contain git_tag_create_buffer function.
func (c *TagsCollection) CreateTagBuffer(name string, obj Objecter, tagger *Signature, message string) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("object " + obj.AsObject().Id().String() + "\n")
	buf.WriteString("type " + strings.ToLower(obj.AsObject().Type().String()) + "\n")
	buf.WriteString("tag " + name + "\n")
	buf.WriteString("tagger " + tagger.ToString(true) + "\n\n")
	if !strings.HasSuffix(message, "\n") {
		buf.WriteString(message + "\n")
	} else {
		buf.WriteString(message)
	}

	return buf.Bytes()
}

// CreateTagWithSignature creates a tag object from the given contents and
// signature.
func (c *TagsCollection) CreateTagWithSignature(
	tagContent, signature string, force bool,
) (*Oid, error) {
	if !strings.HasSuffix(signature, "\n") {
		signature += "\n"
	}

	tagContent += signature

	cTagContent := C.CString(tagContent)
	defer C.free(unsafe.Pointer(cTagContent))
	cForce := cbool(force)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	oid := new(Oid)
	ret := C.git_tag_create_from_buffer(oid.toC(), c.repo.ptr, cTagContent, cForce)

	runtime.KeepAlive(c)
	runtime.KeepAlive(oid)
	if ret < 0 {
		return nil, MakeGitError(ret)
	}

	return oid, nil
}

func (c *TagsCollection) Remove(name string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	ret := C.git_tag_delete(c.repo.ptr, cname)
	runtime.KeepAlive(c)
	if ret < 0 {
		return MakeGitError(ret)
	}

	return nil
}

// CreateLightweight creates a new lightweight tag pointing to an object
// and returns the id of the target object.
//
// The name of the tag is validated for consistency (see git_tag_create() for the rules
// https://libgit2.github.com/libgit2/#HEAD/group/tag/git_tag_create) and should
// not conflict with an already existing tag name.
//
// If force is true and a reference already exists with the given name, it'll be replaced.
//
// The created tag is a simple reference and can be queried using
// repo.References.Lookup("refs/tags/<name>"). The name of the tag (eg "v1.0.0")
// is queried with ref.Shorthand().
func (c *TagsCollection) CreateLightweight(name string, obj Objecter, force bool) (*Oid, error) {

	oid := new(Oid)

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	o := obj.AsObject()
	err := C.git_tag_create_lightweight(oid.toC(), c.repo.ptr, cname, o.ptr, cbool(force))
	runtime.KeepAlive(c)
	runtime.KeepAlive(obj)
	if err < 0 {
		return nil, MakeGitError(err)
	}

	return oid, nil
}

// List returns the names of all the tags in the repository,
// eg: ["v1.0.1", "v2.0.0"].
func (c *TagsCollection) List() ([]string, error) {
	var strC C.git_strarray

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ecode := C.git_tag_list(&strC, c.repo.ptr)
	runtime.KeepAlive(c)
	if ecode < 0 {
		return nil, MakeGitError(ecode)
	}
	defer C.git_strarray_dispose(&strC)

	tags := makeStringsFromCStrings(strC.strings, int(strC.count))
	return tags, nil
}

// ListWithMatch returns the names of all the tags in the repository
// that match a given pattern.
//
// The pattern is a standard fnmatch(3) pattern http://man7.org/linux/man-pages/man3/fnmatch.3.html
func (c *TagsCollection) ListWithMatch(pattern string) ([]string, error) {
	var strC C.git_strarray

	patternC := C.CString(pattern)
	defer C.free(unsafe.Pointer(patternC))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ecode := C.git_tag_list_match(&strC, patternC, c.repo.ptr)
	runtime.KeepAlive(c)
	if ecode < 0 {
		return nil, MakeGitError(ecode)
	}
	defer C.git_strarray_dispose(&strC)

	tags := makeStringsFromCStrings(strC.strings, int(strC.count))
	return tags, nil
}

// TagForeachCallback is called for each tag in the repository.
//
// The name is the full ref name eg: "refs/tags/v1.0.0".
//
// Note that the callback is called for lightweight tags as well,
// so repo.LookupTag() will return an error for these tags. Use
// repo.References.Lookup() instead.
type TagForeachCallback func(name string, id *Oid) error
type tagForeachCallbackData struct {
	callback    TagForeachCallback
	errorTarget *error
}

//export tagForeachCallback
func tagForeachCallback(name *C.char, id *C.git_oid, handle unsafe.Pointer) C.int {
	payload := pointerHandles.Get(handle)
	data, ok := payload.(*tagForeachCallbackData)
	if !ok {
		panic("could not retrieve tag foreach CB handle")
	}

	err := data.callback(C.GoString(name), newOidFromC(id))
	if err != nil {
		*data.errorTarget = err
		return C.int(ErrorCodeUser)
	}

	return C.int(ErrorCodeOK)
}

// Foreach calls the callback for each tag in the repository.
func (c *TagsCollection) Foreach(callback TagForeachCallback) error {
	var err error
	data := tagForeachCallbackData{
		callback:    callback,
		errorTarget: &err,
	}
	handle := pointerHandles.Track(&data)
	defer pointerHandles.Untrack(handle)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ret := C._go_git_tag_foreach(c.repo.ptr, handle)
	runtime.KeepAlive(c)
	if ret == C.int(ErrorCodeUser) && err != nil {
		return err
	}
	if ret < 0 {
		return MakeGitError(ret)
	}

	return nil
}
