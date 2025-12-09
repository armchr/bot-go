package signals

import (
	"context"

	"bot-go/internal/model/ast"
	"bot-go/pkg/lsp/base"
)

// SignalCategory represents the category of a signal
type SignalCategory string

const (
	CategorySize         SignalCategory = "size"
	CategoryComplexity   SignalCategory = "complexity"
	CategoryCohesion     SignalCategory = "cohesion"
	CategoryCoupling     SignalCategory = "coupling"
	CategoryMessageChain SignalCategory = "message_chain"
	CategoryChange       SignalCategory = "change"
	CategoryEntropy      SignalCategory = "entropy"
	CategoryComposite    SignalCategory = "composite"
)

// SignalScope defines what entity the signal applies to
type SignalScope string

const (
	ScopeClass  SignalScope = "class"
	ScopeMethod SignalScope = "method"
	ScopeFile   SignalScope = "file"
)

// SignalMetadata contains metadata about a signal
type SignalMetadata struct {
	Name        string         // e.g., "LOC", "TCC"
	FullName    string         // e.g., "Lines of Code", "Tight Class Cohesion"
	Category    SignalCategory // Category of the signal
	Scope       SignalScope    // What entity the signal applies to
	Description string         // Human-readable description
	Unit        string         // e.g., "lines", "ratio", "count"
	LowerBetter bool           // true if lower values indicate better quality
	Threshold   *float64       // optional default threshold
}

// Signal is the base interface for all signals
type Signal interface {
	// Metadata returns information about this signal
	Metadata() SignalMetadata

	// Dependencies returns names of signals this signal depends on
	Dependencies() []string
}

// ClassSignal computes a signal value for a class
type ClassSignal interface {
	Signal
	ComputeClass(ctx context.Context, classInfo *ClassInfo, sctx *SignalContext) (SignalResult, error)
}

// MethodSignal computes a signal value for a method
type MethodSignal interface {
	Signal
	ComputeMethod(ctx context.Context, methodInfo *MethodInfo, sctx *SignalContext) (SignalResult, error)
}

// FileSignal computes a signal value for a file
type FileSignal interface {
	Signal
	ComputeFile(ctx context.Context, fileInfo *FileInfo, sctx *SignalContext) (SignalResult, error)
}

// AggregateSignal can aggregate method-level values to class-level
type AggregateSignal interface {
	Signal
	Aggregate(ctx context.Context, methodResults []SignalResult) (SignalResult, error)
}

// DependentSignal requires other signals to be computed first
type DependentSignal interface {
	Signal
	// ComputeWithDependencies receives pre-computed dependency results
	ComputeWithDependencies(ctx context.Context, target interface{}, deps map[string]SignalResult, sctx *SignalContext) (SignalResult, error)
}

// Visibility represents field/method visibility
type Visibility string

const (
	VisibilityPublic    Visibility = "public"
	VisibilityPrivate   Visibility = "private"
	VisibilityProtected Visibility = "protected"
	VisibilityPackage   Visibility = "package"
)

// AccessType indicates read or write access
type AccessType string

const (
	AccessTypeRead  AccessType = "read"
	AccessTypeWrite AccessType = "write"
)

// ClassInfo contains all information about a class needed for signal computation
type ClassInfo struct {
	// Identity
	NodeID   ast.NodeID
	Name     string
	FilePath string
	FileID   int32

	// Location
	Range base.Range

	// Structure
	Methods []*MethodInfo
	Fields  []*FieldInfo

	// Relationships
	ParentClasses  []ast.NodeID // Inherited classes
	ChildClasses   []ast.NodeID // Classes that inherit from this
	ContainingFile ast.NodeID

	// Source code (optional, loaded on demand)
	//SourceCode string

	// Precomputed helpers
	accessorMethods map[ast.NodeID]bool // Cache of accessor method IDs
}

// MethodInfo contains all information about a method
type MethodInfo struct {
	// Identity
	NodeID      ast.NodeID
	Name        string
	ClassName   string
	ClassNodeID ast.NodeID
	FilePath    string
	FileID      int32

	// Location
	Range base.Range

	// Signature
	Parameters []*ParameterInfo
	ReturnType string

	// Structure
	LocalVariables []*VariableInfo
	FunctionCalls  []*FunctionCallInfo
	FieldAccesses  []*FieldAccessInfo

	// Control flow
	Conditionals []ast.NodeID
	Loops        []ast.NodeID
	Blocks       []ast.NodeID

	// Source code (optional)
	//SourceCode string

	// Flags
	IsAccessor    bool
	IsConstructor bool
	IsStatic      bool
}

// FieldInfo contains information about a class field
type FieldInfo struct {
	NodeID      ast.NodeID
	Name        string
	Type        string
	ClassNodeID ast.NodeID
	Range       base.Range
	Visibility  Visibility
	IsStatic    bool
}

// FileInfo contains information about a source file
type FileInfo struct {
	NodeID   ast.NodeID
	Path     string
	Language string
	FileID   int32
	Range    base.Range

	// Structure
	Classes   []*ClassInfo
	Functions []*MethodInfo // Top-level functions

	// Source code
	// SourceCode string
}

// ParameterInfo contains information about a method parameter
type ParameterInfo struct {
	NodeID   ast.NodeID
	Name     string
	Type     string
	Position int
}

// VariableInfo contains information about a local variable
type VariableInfo struct {
	NodeID ast.NodeID
	Name   string
	Type   string
	Range  base.Range
}

// FunctionCallInfo contains information about a function call
type FunctionCallInfo struct {
	NodeID         ast.NodeID
	Name           string
	TargetMethodID ast.NodeID // Resolved target (if available)
	TargetClassID  ast.NodeID // Class containing target method
	ChainLength    int        // Message chain length
	Arguments      []ast.NodeID
	Range          base.Range
}

// FieldAccessInfo contains information about a field access
type FieldAccessInfo struct {
	NodeID       ast.NodeID
	FieldName    string
	FieldNodeID  ast.NodeID
	OwnerClassID ast.NodeID
	IsForeign    bool       // true if accessing field from another class
	AccessType   AccessType // read or write
}

// NewClassInfo creates a new ClassInfo instance
func NewClassInfo(nodeID ast.NodeID, name string, filePath string, fileID int32) *ClassInfo {
	return &ClassInfo{
		NodeID:          nodeID,
		Name:            name,
		FilePath:        filePath,
		FileID:          fileID,
		Methods:         make([]*MethodInfo, 0),
		Fields:          make([]*FieldInfo, 0),
		ParentClasses:   make([]ast.NodeID, 0),
		ChildClasses:    make([]ast.NodeID, 0),
		accessorMethods: make(map[ast.NodeID]bool),
	}
}

// GetMethods returns all methods
func (c *ClassInfo) GetMethods() []*MethodInfo {
	return c.Methods
}

// GetNonAccessorMethods returns methods excluding accessors/mutators
func (c *ClassInfo) GetNonAccessorMethods() []*MethodInfo {
	return nil
}

// GetAccessorMethods returns only accessor/mutator methods
func (c *ClassInfo) GetAccessorMethods() []*MethodInfo {
	return nil
}

// GetFields returns all fields
func (c *ClassInfo) GetFields() []*FieldInfo {
	return c.Fields
}

// GetPublicFields returns only public fields
func (c *ClassInfo) GetPublicFields() []*FieldInfo {
	return nil
}

// GetLOC returns lines of code
func (c *ClassInfo) GetLOC() int {
	return 0
}

// IsAccessorMethod checks if a method is an accessor/mutator
func (c *ClassInfo) IsAccessorMethod(methodID ast.NodeID) bool {
	return false
}

// NewMethodInfo creates a new MethodInfo instance
func NewMethodInfo(nodeID ast.NodeID, name string, classNodeID ast.NodeID) *MethodInfo {
	return &MethodInfo{
		NodeID:         nodeID,
		Name:           name,
		ClassNodeID:    classNodeID,
		Parameters:     make([]*ParameterInfo, 0),
		LocalVariables: make([]*VariableInfo, 0),
		FunctionCalls:  make([]*FunctionCallInfo, 0),
		FieldAccesses:  make([]*FieldAccessInfo, 0),
		Conditionals:   make([]ast.NodeID, 0),
		Loops:          make([]ast.NodeID, 0),
		Blocks:         make([]ast.NodeID, 0),
	}
}

// GetLOC returns lines of code
func (m *MethodInfo) GetLOC() int {
	return 0
}

// GetParameterCount returns number of parameters
func (m *MethodInfo) GetParameterCount() int {
	return 0
}

// GetLocalVariableCount returns count of local variables (excluding params)
func (m *MethodInfo) GetLocalVariableCount() int {
	return 0
}

// GetFunctionCallCount returns count of function calls
func (m *MethodInfo) GetFunctionCallCount() int {
	return 0
}

// GetDecisionPointCount returns count of decision points (for cyclomatic)
func (m *MethodInfo) GetDecisionPointCount() int {
	return 0
}

// NewFieldInfo creates a new FieldInfo instance
func NewFieldInfo(nodeID ast.NodeID, name string, classNodeID ast.NodeID) *FieldInfo {
	return &FieldInfo{
		NodeID:      nodeID,
		Name:        name,
		ClassNodeID: classNodeID,
	}
}

// NewFileInfo creates a new FileInfo instance
func NewFileInfo(nodeID ast.NodeID, path string, language string, fileID int32) *FileInfo {
	return &FileInfo{
		NodeID:    nodeID,
		Path:      path,
		Language:  language,
		FileID:    fileID,
		Classes:   make([]*ClassInfo, 0),
		Functions: make([]*MethodInfo, 0),
	}
}
