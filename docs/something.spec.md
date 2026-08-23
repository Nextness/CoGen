# Something Spec - General Information

SOMETHING stands for **Simple Orchestration Markup for Expression, Transformation, and Hierarchical Instruction Notation Generation** and uses the file format `<filename>.something`.

This is a configuration format. This spec defines what SOMETHING is and the syntax. There are no forward declarations. Names become visible at their declaration position and remain visible for the rest of that lexical scope. `#priv` changes whether a destination is published, not whether later language statements can access it.

This is a top-to-bottom, left-to-right configuration file. A reference to a value, type, or macro before its declaration is an error.

## Compilation model

SOMETHING is processed in five ordered stages:

1. The lexer produces source-aware tokens.
2. The parser produces a source-ordered syntax AST. It records directives without executing them and does not resolve names or check types.
3. Directive generation expands `#include`, `#for`, `#insert`, `#iteration`, `#as_lvalue`, and macro calls into the existing AST at their source positions. Directive inputs may be checked and temporarily evaluated as needed for expansion. Assertion and validation directives (`#assert`, `#if`, `#error`) and expression operators (`#not`, `#and`, `#or`, `#match`, `#len`, `==`, `!=`, `<`, `<=`, `>`, `>=`) pass through directive generation unchanged and are evaluated during the type-checking and evaluation stages.
4. The type checker validates the complete expanded program in source order.
5. The evaluator executes only the checked, directive-free AST and publishes non-private destinations.

The language does not support recursion. Circular includes, direct or indirect macro recursion, recursive setup or enum types, and circular value dependencies are errors. Implementations should report the dependency chain when it is available.

# Comments

SOMETHING has two types of comments `//` for commenting the entire line and `/** **/` for block comments, which can be nested.

# Builtin Types

## Primitive Types

- string:
    - Literals use double quote: `"<whatever>"`;
    - Literals use single quote: `'<whatever>'`;
    - Multiline are allowed using a directive (check the Multiline Directive section): `#multiline IO <whatever> IO`;
    - Strings also allow you to interpolate values. Take a look at how interpolation and variables works.
- integer:
    - Literals like: `1`, `2`, `3`, `4`, `-1`, `0`, `-2`, `1E10`, `1_000` (1000), `1_0` (10);
- float:
    - Literals like: `0.1`, `0.2`, `0.11111`, `1E-10`, `100.123`, `0.11_00` (0.11), `10.0`, `20.1234`, `0.00001`;
- timestamp:
    - Act like strings, but have the format `year-month-day hour:minute:seconds.microseconds`, for example `"2026-01-01 22:10:01.002"` or `"2026-01-01 22:10:01"`; microseconds are optional, and the separator between date and time is one space rather than `T` or another separator.
- boolean:
    - Literals like: `true`, `false`.

## Compound Types

- scope:
    - It defines a scope within the file. Check for the scope definition section for more details;
    - This is only a type. We cannot have an expression for it;
- namespace:
    - It defines a namespace for an included file. Check for the namespace definition section for more details;
    - This is only a type. We cannot have an expression for it;
- setup:
    - It defines a new type, similar to struct in C or Rust. Check for the setup definition section for more details;
    - This is only a type. We cannot have an expression for it;
- array:
    - Defined like `[]<any_type>`;
    - Literals like: `[]string{}`, `[]integer{}`, `[]float{}`, `[]timestamp{}`, `[]boolean{}`, `[]<custom_setup>{}` (custom type);
    - Arrays have special cases:
        - we can define how we want to index the array. If it is left blank, the default index is integer, therefore `[]string` is equivalent to `[integer]string`. This applies for all types;
        - we can also index by enums, so if we have an enum named `some_enum` we could do `[some_enum]string`. That means that we can only index by elements of the enum and not by integer;
- mapping:
    - Defined like `mapping(<type>, <type>)`, where it accepts two type parameters which are used to typecheck that the data in the mapping is correct;
    - Literals like: `mapping(string, string){["something"] => "another_string"}`. Some examples of valid mappings: `mapping(string, boolean)`, `mapping(integer, string)`, `mapping(timestamp, float)`, and in general `mapping(<primitive_type>, ...)` (as long as it agrees with the next item below);
        - For the key, it should work with all the primitive types only. For the value, it should work with all primitive values + scope + namespace + array + mapping;
- enum:
    - Special mechanism to index things and give additional context to the index;
    - Defined like: `enum` or `enum(<type>)`. If defined only with `enum` it acts as only index - e.g. `enum {item_1; item_2; item_3;}` would be the same as syntactically `enum.item_1=0, enum.item_2=1, enum.item_3=2`. On the other hand if we use `enum(<type>)` it acts as a tagged enum - e.g. `enum(string) {item_1 = "a"; item_2 = "b"; item_3 = "c";}` would be the same as syntactically `enum.item_1=0, enum.item_2=1, enum.item_3=2` + `enum.item_1.value="a", enum.item_2.value="b", enum.item_3.value="c"`;
        - This should work for all the primitive types + setup + mapping + array.

## Scope Definition

Scopes are a way for us to centralize a series of statements into a single location that is not shared with the global scope. However, if you do need to use these variables at a later time, you can access them using the scope name. Check Expressions and Statements sections for details.

Also for accessing members, check the section Access Notations.

```something
scope_name_not_shared: scope = {
    // These variables only exist in here. Once we go out of the scope they no longer exists.
    variable_name: string = "some_input.txt";
    another_variable: integer = 100;
    result: string = "{another_variable}_{variable_name}";
}

some_other_step_later: string = scope_name_not_shared.result;
```

As mentioned earlier, `scope` is just a way to specify a type. We don't have an expression like `scope {}`.

## Access Notations

There are three ways to access members of compound elements. They can have any combinations between them. As long as the implementation can resolve it, it should be valid. The following three subsections contain the explanation for each access notation depending on the type.

### Dot Access Notation for Members

Dot is used in four different scenarios. The first is to access elements inside a scope. The second is to access a field on a setup instance. The third is to access an enum variant explicitly. The fourth is to access the underlying value of a tagged enum variant (`.value`). See the examples below.

When accessing an enum variant, the qualified form `some_enum.opt1` is always valid. The shorthand `.opt3` (without the type name) is also valid in any position where the expected type is already known from context — for example, in a variable assignment where the left-hand side declares the enum type, or as an array/map index where the index type is already fixed by the array or map type declaration. If the enum type cannot be inferred from context, the qualified form must be used.

```
// Example 1
some_scope: scope = {
    a: string = "";
    b: boolean = true;
}
c: boolean = some_scope.b;

// Example 2
example_setup: setup = {
    field1: string;
    field2: integer;
}
example: example_setup = {
    field1 = "whatever",
    field2 = 10,
};
d: string = example.field1;

// Example 3
some_enum: enum = {
    opt1;
    opt2;
    opt3;
}
a: some_enum = .opt3;
b: some_enum = some_enum.opt1;
c := some_enum.opt2;

// Example 4: Tagged enum .value accessor
tagged_enum: enum(string) = {
    x = "alpha";
    y = "beta";
}
d: string = tagged_enum.x.value; // results in "alpha"
```

### Array Access Notation for Items

For array access notation, we need to use open and close brackets, and in between we need either use the integer that represents the index of the element we want to access or the enum in which we are indexing the array. If we use index based (integer), it starts at 0.

```something
variable_array_1 := []string {
    "a", "b", "c"
};
example_1: string = variable_array_1[0];  // result should be "a"
// error_var: string = variable_array_1[4]; // Out of bounds - implementation should result in error

some_enum: enum = {
    option1;
    option2;
    option3;
}
variable_array_2 := [some_enum]string {
    "a", "b", "c"
};
example_2: string = variable_array_2[.option1]; // result should be "a"
// error_var_2: string = variable_array_2[.option4]; // invalid enum option - implementation should result in error since it cannot index by something that doesn't exist
```

### Map Access Notation for Elements

For map access notation, we need to use open and close brackets, and in between whatever key we use to index that map, so that we can get the value.

```
variable_map := mapping(string, integer) {
    ["a"] => 1,
    ["b"] => 2,
    ["c"] => 3,
};
something_else: integer = variable_map["a"];
```

### Complex examples of access notation

```something
some_enum: enum(string) = {
    opt1 = "x";
    opt2 = "y";
    opt3 = "z";
}

some_setup: setup = {
    field1: string;
    field2: mapping(string, boolean);
    field3: []timestamp;
    field4: some_enum;
}

another_setup: setup = {
    input1: some_setup;
    input2?: string = "";
    input3: float;
}

example := some_setup {
    field1 = "hello",
    field2 = mapping(string, boolean) {
        ["a"] => true,
        ["b"] => true,
        ["c"] => false,
    },
    field3 = []timestamp{
        "2022-10-11 15:16:10.333",
        "2023-10-11 15:16:10.333",
        "2024-10-11 15:16:10.333",
        "2025-10-11 15:16:10.333",
    },
    field4 = .opt3,
};

another := another_setup {
    input1 = example,
    input3 = 10.1,
};

a := another.input1.field1;
b := another.input1.field2["a"];
c := another.input1.field3[2];
d := another.input1.field4;
e := another.input1.field4.value;
```

## Namespace Definition

Namespace is a way for us to put a bunch of variables, setups, scopes, and in general everything already defined, behind a namespace when we include other files into our configuration. This is used with the include directive - take a look at the include directive for more details. Check Expressions and Statements sections for details.

```something
// This includes files without a namespace
#include("<filename>.something");

// This includes files with a namespace
fn: namespace = #include("<filename>.something");

// This includes files with a namespace but inferred
fn := #include("<filename>.something");
```

## Setup Definition

Setup is a way for us to create a new type and use it multiple times across the configuration - similar to how C or Rust structs work. They are also used to type check everything is properly configured. Check Expressions and Statements sections for details.

```something
new_type: setup = {
    input1: string;
    input2: boolean;
    input3: timestamp;
}

another_type: setup = {
    field_1: new_type;
    field_2?: string = "";
    field_3?: float = 0.0;
    field_4? := true;
}
```

As you can see, it is pretty easy to specify new datatypes. Also we can nest setups. Also consider that we can have optional arguments, which REQUIRE a default value - this means that when we instantiate this setup, if we don't provide the field, it will use the default argument - having to use '?' and then set the value is intentional. Type inference also works with the setup fields only when we have default values - i.e. using '?'.

```
new_type_instance := new_type {
    input1 = "some_value",
    input2 = true,
    input3 = "2024-02-18 13:50:15.900",
};

another_type_instance: another_type = {
    field_1 = new_type_instance,
};
```

As you can see, for new data types, we can have them as expressions, we can skip defining certain field because they have defaults and look pretty simple. All these are typechecked to make sure there is no mistake.

### Anonymous struct literals in typed contexts

When the expected type of a struct literal is already known from context (for instance, when it appears as an element of an array typed with a known setup type (`[]some_setup`), or as a value in a mapping typed with a known setup type) the setup name may be omitted and the braces used directly. The type is inferred from the enclosing field's type definition.

```
nested_type: setup = {
    source: string;
    file_type: string;
    date: timestamp;
}

parent: setup = {
    sources: []nested_type;
}

// Each anonymous { ... } inside the array inherits the element type "nested_type"
// from the field declaration "sources: []nested_type".
instance := parent {
    sources = [
        { source = "scopus", file_type = "csv", date = "2026-06-07 22:18:00.000" },
        { source = "wos", file_type = "bib", date = "2026-06-07 22:18:00.000" },
    ],
};
```

This applies to any position where the expected type is fully determined by a type annotation - including array elements, mapping values, and field assignments in a parent setup literal.

## Variable Interpolation for String

SOMETHING takes a very simple approach for string interpolation. Since all variables and values are known, we can simply use the format open and close braces inside quotes. The following are valid:

```something
tmp_a: string = "some value in here";
tmp_b := 10.0;
local_scope: scope = {
    tmp_c := true;
    tmp_d := "2012-08-11 17:42:15.709";
}

interpolation_1: string = 'literal string: {tmp_a} {tmp_b}';         // results in "literal string: some value in here 10.0"
interpolation_2: string = "{interpolation_1} + {local_scope.tmp_c}"; // results in "literal string: some value in here 10.0 + true"
```

# Expressions

All the examples mentioned in the Types section represent a type of expression. In this language, we can define a expression the following way:

```
expr ::= "string literal"
       | 'string literal'
       | #multiline EOF ... EOF
       | #include
       | 1
       | 10
       | 1E10
       | 0.1
       | 0E-10
       | 100_000
       | 0.00_1
       | true
       | false
       | "2026-10-02 22:11:01.022"
       | []<type>{ ... }
       | mapping(<type>, <type>) { ... }
       | enum {}
       | enum(<type>) { ... }
       | <defined_setup> { ... }
       | <macro_name>!(<args>) // Macro call
       | { ... } // Inferred: resolves a scope literal (if we have something like `val: scope = { ... }`), setup literal, or a compound type literal
       | <expr> == <expr>      // equality
       | <expr> != <expr>      // inequality
       | <expr> < <expr>       // less than
       | <expr> <= <expr>      // less than or equal
       | <expr> > <expr>       // greater than
       | <expr> >= <expr>      // greater than or equal
       | #not <expr>           // boolean negation
       | <expr> #and <expr>    // boolean AND (short-circuit)
       | <expr> #or <expr>     // boolean OR (short-circuit)
       | #match(<expr>, <expr>) // regex match; returns boolean
       | #len(<expr>)          // length of array or mapping; returns integer
       | (<expr>)              // grouping / precedence override
```

### Operator precedence (lowest to highest)

```
  #or                       (infix, lowest)
  #and                      (infix)
  ==  !=                    (infix)
  <  <=  >  >=              (infix)
  #not                      (prefix)
  #match()  #len()          (call-like)
  (expr)                    (grouping, highest)
```

Parentheses override the default precedence. For example, `#not (format_version >= min_version #and format_version <= max_version)` groups the comparisons before applying `#and` and `#not`.

### Short-circuit evaluation

`#and` and `#or` use short-circuit evaluation (left to right):
- For `#and`, if the left operand is `false`, the right operand is not evaluated.
- For `#or`, if the left operand is `true`, the right operand is not evaluated.

### Type checking for comparisons

Comparison operators (`==`, `!=`, `<`, `<=`, `>`, `>=`) require both operands to have the same type. Comparing different types (e.g., string vs integer) is a type error with an explanation. Relational operators (`<`, `<=`, `>`, `>=`) additionally require comparable types (integer, float, string, timestamp).

### String escaping

To include literal `{` or `}` characters inside a string literal (both double-quoted and single-quoted), use `{{` for `{` and `}}` for `}`. This escaping applies in all string expressions, including `#error("...")` messages. The lexer converts `{{` to `{` and `}}` to `}` before processing interpolation references.

Inside multiline strings, `\/` is an escape for a literal `/`, so `\/\/` produces `//` without starting a comment. Regular string literals do not treat `\/` specially.

# Statements

Using the expressions defined previously, complete statements use a common assignment model. A destination can be a variable, an existing member or index, or a directive-generated lvalue. Enum and setup declarations are specialized typed assignments.

```
stmt ::= <name> : enum = { <enum_members> }                           // enum declaration
       | <name> : enum(<type>) = { <tagged_enum_members> }            // typed enum declaration
       | <name> : setup = { <field_definitions> }                     // setup declaration
       | <name> : scope = { <stmt>* }                                 // scope declaration
       | #macro <name> := (<typed_params>) -> <type> { <macro_body> } // macro declaration
       | <variable_name> : <type> = <expr>;                           // explicit type declaration
       | <variable_name> := <expr>;                                   // type inference based on the expression
       | <existing_lvalue> = <expr>;                                  // reassignment
       | #for <ident>: <array> { stmt }
       | #for <key>, <val>: <mapping> { stmt }
       | #include(<string_literal>);
       | #insert { [<string_literal_or_multiline> ("," <string_literal_or_multiline>)* [","]] };
       | #iteration: <type> = <expr>;
       | #iteration("_<label>"): <type> = <expr>;
       | #iteration("_<label>") := <expr>;
       | <variable_name> := #iteration;
       | <variable_name> := #iteration("_<label>");
       | <variable_name> : string = #iteration("_<label>");
       | #as_lvalue(<value_name>) := <expr>;
       | #as_lvalue(<value_name>): <type> = <expr>;
       | #as_lvalue(<value_name>) = <expr>;
       | #assert <setup_type_name> { <stmt>* }                        // setup type assertion
       | #if <expr> { <stmt>* }                                       // conditional block
       | #if <expr> <stmt>;                                           // conditional single statement
       | #error(<string_expr>);                                       // user-defined compilation error
```

## Required terminators and separators

Semicolons and commas are grammar tokens, not optional formatting. A missing required token and an extra token after a construct that does not take one are both syntax errors.

- Explicit declarations, inferred declarations, reassignments, setup literals, array literals assigned as statements, mapping literals assigned as statements, macro-call assignments, `#priv` assignments, `#include`, `#insert`, `#iteration`, `#as_lvalue`, `#error`, and assignments whose value is `#multiline` require a final semicolon.
- Scope declarations, setup definitions, enum definitions, `#for` directives, macro definitions, `#assert` directives, and `#if` block directives do not take a semicolon after their closing brace. Writing one is an error.
- Every setup field definition requires a semicolon, including the last field.
- Every enum member requires a semicolon, including the last member.
- Setup-literal fields, array elements, mapping entries, and `#insert` strings require commas between elements. A trailing comma before the closing delimiter is optional. Semicolons cannot replace these commas.
- `#set` requires a semicolon after its expression. The macro definition itself does not take a semicolon.
- `#insert {};` is valid and expands to no statements.
- `#error` requires a semicolon after its closing parenthesis.
- `#if` with a single-statement form: the inner statement's terminator (e.g., `;`) terminates the construct. No additional semicolon is added after the single statement.

```something
settings: scope = {
    value := 1;
}

Record: setup = {
    name: string;
    weight?: float = 0.1;
}

record: Record = {
    name = "example",
    weight = 0.5,
};

Kind: enum = { A; B; C; }
TaggedKind: enum(string) = { A = "A"; B = "B"; C = "C"; }

names := []string{"a", "b", "c"};
aliases := mapping(string, string){["a"] => "A", ["b"] => "B",};
#insert {};
```

## Declaration and reassignment

`name: type = value` and `name := value` declare a new destination in the current lexical scope. Declaring the same name twice in one scope is an error. A nested scope may declare the same name independently.

`lvalue = value` reassigns a destination that has already been declared. Reassignment follows C-like type stability: the new value must be assignable to the destination's existing type. It supports variables, scope and setup members, existing array indices, and existing mapping keys. Reassignment does not add a missing mapping key or create a new field.

```something
value := 1;
value = 2;

holder: scope = {
    value := "outer member";
    nested: scope = {
        value := "nested member";
        copy := holder.value;
    }
}

holder.value = "reassigned";
items := []integer{1, 2};
items[0] = 3;
values := mapping(string, integer){["a"] => 1};
values["a"] = 2;
```

# Directives

As part of the language, SOMETHING has several directives that are used to improve organization, replication and overall display of information for configuring whatever you want. All directives start with hash '#' followed by the directive name.

Some directives act like expressions, but some act like statements.

The parser stores directives in the syntax AST. Directive generation executes them before whole-program type checking and replaces them with ordinary AST assignments and expressions. Generated declarations obey the same source-order and duplicate-name rules as handwritten declarations. For example, a `#for` body that writes the same destination on every iteration must either reassign a declaration made before the loop with `=` or generate distinct lvalues with `#iteration` or `#as_lvalue`.

## Multiline Directive

- Directive name: `#multiline`;
- Directive description: It works like a bash script multiline. Once we define the directive and the arguments, we need to provided a delimiter which will be used to track where the multiline starts and where it ends. Similar to a normal string, it also can interpolate string and variables. Check it at the variable section.
- Requires arguments to work: NO;
- Accepts arguments: YES;
    - no_newline: if set, it will remove newlines automatically;
    - no_indent: if set, it will remove indentation automatically;
    - strip_spaces: if set, it will remove whitespaces between each line automatically;
- Comments: A multiline body may contain `//` comments. An unescaped `//` starts a comment that runs to the end of its line, and the comment is removed from the resulting string value. This applies to every body line including the line that closes the literal, so a closing delimiter may carry a trailing comment. To include a literal `//` in the value, escape both slashes as `\/\/` (a single `\/` is an escape for a literal `/`).
- Directive usage:
    ```something
    // Example no arguments
    variable := #multiline EOF
    Anything here will
    retsult in a multiline string, that we can use for
    later if we want.
    EOF;

    // Example with one argument
    variable := #multiline (no_newline) EOF
    Anything here will
    retsult in a multiline string, that we can use for
    later if we want.
    EOF;

    // Example with multiple arguments
    variable := #multiline (no_newline|no_indent) EOF
    Anything here will
    retsult in a multiline string, that we can use for
    later if we want.
    EOF;
    ```

## Include Directive

- Directive name: `#include`;
- Directive description: It works like an include in C, in the sense that it includes the information of another file into the current file. However it has the option to namespace it, avoiding name collisions. Additionally, we can use it wherever we need - either at global scope or local scope. If the same file is included more than once, the implementation must handle it by loading it only once (similar to `#pragma once`). Deduplication is by file path only, ignoring namespace - meaning if the same file is included with different namespaces, it is still loaded once but exposed under each namespace separately. If a user includes the same file with different namespaces and that causes slow loads or other issues, it is a user-level configuration problem, not an implementation bug.
- Requires arguments to work: YES;
- Accepts arguments: YES;
    - string literals that represent a filepath that is valid;
- Circular include chains are invalid. The implementation reports the active include chain instead of relying on include deduplication to hide recursion.
- Directive usage:
    ```something
    // At global scope no namespace
    #include("<valid_file>.something");

    // At global scope with namespace
    val: namespace = #include("<valid_file>.something");

    // At global scope with namespace and inferred
    val := #include("<valid_file>.something");

    // At local scope
    local_considerations: scope = {
        // All formats should work - no namespace, with namespace, and inferred namespace.
        val := #include("<valid_file>.something");
    }
    ```

## For Directive

- Directive name: `#for`;
- Directive description: It works like a loop. It allows us to dynamically loop maps and arrays to generate new configuration more easily than just copy and paste. In general, we can use whatever we want to (and as deep as possible - like nested scopes with nested setups in mappings and arrays - as long as we can resolve it and typechecks properly, it should work).
- Requires arguments to work: YES;
- Accepts arguments: YES;
    - If we loop an array we need an identifier to use in the loop scope;
    - If we loop a mapping we need key and value identifiers to use in the loop scope;
- Directive usage:
    ```something
    // Arrays at the global scope
    array_literal := []string{"a", "b", "c"};
    #for element: array_literal {
        ...
    }

    // Maps at the global scope
    map_literal := mapping(string, string) {["a"] => "b", ["c"] => "d"};
    #for key, val: map_literal {
        ...
    }

    some_local_scope: scope = {
        local_array_literal := []string{"a", "b", "c"};
        local_map_literal := mapping(string, string) {["a"] => "b", ["c"] => "d"};
    }

    #for element: some_local_scope.local_array_literal {
        ...
    }
    #for key, element: some_local_scope.local_map_literal {
        ...
    }
    ```

Array iterations are expanded in element order. Mapping iterations are expanded in deterministic key order. Loop variables exist only while generating one body expansion and are replaced by their compile-time values in the generated AST.


## Insert Directive

- Directive name: `#insert`;
- Directive description: It works by treating whatever string literal is inside the block as a string that needs to be inserted as configuration code. This allows us to generate code based on previous configuration, without duplication, and using string interpolation. It accepts string literals, multiline strings, and a combination of both, separated by a comma. We can have literally anything that the language supports as a string literal and it will treat it as code.
- Requires arguments to work: NO;
- Accepts arguments: NO;
- Directive usage:
    ```
    // Single string literal
    #insert { "a := 'something';" };

    // Empty insertion is a no-op
    #insert {};

    // Multiple string literals
    #insert {
        "b := true;",
        "c: float = 10.0;",
    };

    // Mix string literals + multiline
    #insert {
        #multiline EOF
        d: scope = {
            e: timestamp = "2021-04-21 09:08:31.301";
        }
        EOF,
        "g := './some/file/path.txt';"
    };
    ```


## Iteration Directive

- Directive name: `#iteration`;
- Directive description: It works as a builtin mechanism for us to track increases on iterations of configurations. This will generate an Lvalue with a specific format `iteration_` + `padded 10 digits starting at 0` + `optional label`. This Lvalue will have different counts depending on the label used, or not used at all. In general, `#iteration` should act like a variable name if it is placed as an Lvalue, or as a value if it is an Rvalue. Because it acts like an Lvalue, we can have pretty much everything as the type and then value for it.
- Requires arguments to work: NO;
- Accepts arguments: YES;
- Directive usage:
    ```something
    #iteration: string = "a"; // iteration_0000000000
    #iteration: string = "b"; // iteration_0000000001
    #iteration: string = "c"; // iteration_0000000002

    #iteration("_label"): string = "d"; // iteration_0000000000_label
    #iteration("_label"): string = "e"; // iteration_0000000001_label
    #iteration("_label"): string = "f"; // iteration_0000000002_label

    #iteration: <primitive_type> | <compound_type> = ...; // Basically everything here works
    use_same_iteration_for_something_1: string = #iteration; // The only caveat here is that when #iteration is a Rvalue, the expected type is string.
    use_same_iteration_for_something_2 := #iteration; // The only caveat here is that when #iteration is a Rvalue, the expected type is string. Infer also should work.
    ```

Directive generation replaces an lvalue `#iteration` with the generated identifier and replaces an rvalue `#iteration` with the next generated identifier as a string. The full type checker sees only those concrete forms.

## AsLvalue Directive

- Directive name: `#as_lvalue`;
- Directive description: It evaluates a string and parses that string as an lvalue;
- Requires arguments to work: YES;
- Accepts arguments: YES;
    - a non-empty string literal or string-valued expression containing an identifier or member/index path;
- Directive usage:
    ```
    some_variable: string = "probably_something";
    #as_lvalue(some_variable): boolean = true;         // probably_something: boolean = true;
    #as_lvalue("another_example"): string = "nothing"; // another_example: string = "nothing";

    holder: scope = { value := 1; }
    member_name := "holder.value";
    #as_lvalue(member_name) = 2; // holder.value = 2;
    ```

## Assert Directive

- Directive name: `#assert`;
- Directive description: Validates a setup type by running assertion statements in a scope that inherits the type's fields. The assertion body is evaluated each time an instance of the asserted setup type is created, with access to that instance's field values. If any `#error` directive is reached during assertion evaluation, compilation fails with the error message and the location of the instance that triggered the failure.
- Requires arguments to work: YES;
- Accepts arguments: NO;
- Directive usage:
    ```something
    Point: setup = { x: integer; y: integer; }
    #assert Point {
        #if x < 0 {
            #error("x must be non-negative, got {x}");
        }
    }
    p := Point { x = -1, y = 5 }; // triggers: ERROR: x must be non-negative, got -1
    ```

## If Directive

- Directive name: `#if`;
- Directive description: Conditionally executes its body when the condition evaluates to `true`. Supports two forms: a block form with multiple statements inside braces, and a single-statement form without braces. The condition must be a boolean expression. Variables assigned inside the body affect the current scope (reassignment with `=`) or create new bindings in the current scope (declaration with `:=`).
- Requires arguments to work: YES;
- Accepts arguments: NO;
- Directive usage:
    ```something
    // Block form
    #if true {
        x := 1;
        y = 2;
    }

    // Single-statement form
    #if false x = 0;
    ```

## Error Directive

- Directive name: `#error`;
- Directive description: Always terminates compilation with a user-defined error message. The message is a string expression that may use interpolation. Typically used inside `#assert` or `#if` bodies to report validation failures.
- Requires arguments to work: YES;
- Accepts arguments: NO;
- Directive usage:
    ```something
    #error("Custom error message");
    #error("Value {some_variable} is invalid");
    ```

## Not, And, Or Operators

- Directive names: `#not`, `#and`, `#or`;
- Directive description: Boolean operators for composing conditions. `#not` is a prefix unary operator that negates its operand. `#and` and `#or` are infix binary operators that use short-circuit evaluation (left to right). All require boolean operands.
- Requires arguments to work: YES;
- Accepts arguments: NO;
- Directive usage:
    ```something
    result := #not true;           // false
    result := true #and false;     // false
    result := false #or true;      // true
    result := #not (a > 5 #and b < 10);
    ```

## Match Directive

- Directive name: `#match`;
- Directive description: Evaluates a regex match against a string value. Returns `true` if the pattern matches the value, `false` otherwise. The pattern must be a valid Go RE2 regular expression. Glob patterns like `*.something` are not valid regex; use `.*\.something` instead.
- Requires arguments to work: YES;
- Accepts arguments: NO;
- Directive usage:
    ```something
    result := #match("hello world", "hello");  // true
    result := #match("hello world", "goodbye"); // false
    result := #match(filename, "^[a-z]+\\.txt$");
    ```

## Len Directive

- Directive name: `#len`;
- Directive description: Returns the length of an array or mapping as an integer. Using `#len` on a non-array, non-mapping value is a type error.
- Requires arguments to work: YES;
- Accepts arguments: NO;
- Directive usage:
    ```something
    items := []string{"a", "b", "c"};
    count := #len(items); // 3

    mapping_value := mapping(string, integer){["a"] => 1, ["b"] => 2};
    count := #len(mapping_value); // 2
    ```

# Macro Definition and Call

Macros are a way to define reusable expressions that expand inline at the call site. They are similar to parameterized constants: a macro defines a `#set` expression that is evaluated in the macro's own scope, and the result replaces the call.

## Macro Declaration

A macro is declared with the `#macro` directive, followed by the name, typed parameters, return type, and a body block. The body block must end with a `#set` directive that introduces the expansion expression.

```something
#macro <name> := (<typed_params>) -> <return_type> {
    // Optional local variables and directives
    #set <expression>;
}
```

The body may contain assignments, directives (`#for`, `#insert`, etc.), and other constructs. These are expanded and type checked in the macro's scope and are available to the `#set` expression, but are not exposed outside the macro. Only the value of the `#set` expression is used as the expansion result.

## Macro Call

A macro is called by writing the macro name followed by `!` and arguments in parentheses:

```something
result := <macro_name>!(<arg1>, <arg2>, ...);
```

The arguments are bound to the macro's parameters in order, then the macro body is expanded and checked, and the `#set` expression value is returned.

## Examples

```something
// Macro with no parameters, returning a string
#macro greeting := () -> string {
    #priv base := "Hello";
    #set "{base}, World!";
}

x: string = greeting!(); // evaluates to "Hello, World!"

// Macro with parameters, returning a list
#macro make_pair := (a: string, b: string) -> []string {
    #set []string { a, b };
}

pair := make_pair!("x", "y"); // evaluates to []string{"x", "y"}

// Macro using setup types
Point: setup = { x: integer; y: integer; }
#macro origin := () -> Point {
    #set Point { x = 0, y = 0 };
}

o := origin!(); // evaluates to Point { x = 0, y = 0 }
```

## Type Checking

Every argument is checked against its declared parameter type. The expanded body is type checked with those typed parameters in scope. The `#set` expression is checked against the declared return type. If any type does not match, the macro call fails with a type error.

## Implementation Notes

- Macros can be declared at the top level or inside scopes and other macro bodies.
- Macro bodies have their own scope; variables defined inside the macro body are not visible outside.
- Macro parameters are bound in the macro's scope before the body is evaluated.
- The `#set` directive can only appear as the final statement inside a macro body.
- Macro declarations are visible only after their source position.
- Direct and indirect macro recursion are invalid. The implementation reports the recursive expansion chain.

# Private Information

Because this is a configuration file, it is expected that some intermediate values are not useful in the final state. `#priv` before an assignment keeps that destination available to later language statements but excludes it from the published result.

Privacy belongs to the destination. Assigning or reassigning from a private source into a public destination publishes the destination. Reassigning a private destination does not make it public. Private members can be referenced through their qualified paths while the program is being expanded, checked, and evaluated.

In the example below, the basepath is not relevant for an imaginary application, only the full path with the files that we need to look at eventually; therefore, instead of having a useless variable we can make it private only to the config evaluation step, instead of available to the end user.

```
#priv a := "./basepath";
b := "file_a.txt";
c := "file_b.txt";

filepaths := []string{
    "{a}/{b}",
    "{a}/{c}"
};
```

Bare `#include` does not create a destination of its own. A namespace assigned from `#include(...)` follows the assignment's normal privacy and publication rules.

# Implementation Considerations

Considerations for who wants to implement a loader for this language.

We have a few different families of functions that need to exist to make this usable in a given language. For the sake of simplicity this document will use a generic structure to define functions, parameters and arguments while trying to show a few examples in different languages.

> **Note:** Do not use regex for parsing or validation anywhere in the implementation. The language grammar is simple enough to handle with a hand-written lexer and parser (recursive descent). Regex makes error reporting, line/col tracking, and composite syntax harder than necessary.

## Private/Internal API

The objective of the private/internal API is to load the `.something` file, tokenize it, parse a source-ordered AST, expand directives, type check the expanded AST, and then evaluate the result into an object or struct useful to an application. A dynamic host can publish generic arrays and mappings after checking. A statically typed host may add a code generation step after the language type checker.

On the other hand, if we are talking about C/C++/Rust/Zig/Java/etc., and the language is not dynamic, we do need a step for codegen where we create structs/classes to handle to evaluated `.something` file.

The private/internal API fail with relevant information for the public API to provide to the user calling the function. For instance:

- If we have a parsing error, we should return the location where we got the error - column, line, filepath, error name, description;
- If we have typechecking error, we should report what is the problem - column, line, filepath, error name, description;
- If the file doesn't exist, give a proper error explaining the file doesn't exist;
- If we trying to access a member that doesn't exist, explain the member is invalid - column, line, filepath, error name, description;
- If we are trying to index by integer instead of enum, explaining that we should use enum indexes, also include the expected enum, what are the values, where it is located for people to check what are the options - column, line, filepath, error name, description;
- If we don't have an identifier, explain that something is not defined, out of scope, or there is a typo - column, line, filepath, error name, description;
- If we don't know the identifier at the `<type>` position explain that we are not aware of that type, it is out of scope, or there is a typo - column, line, filepath, error name, description;
- If we don't know the directives, which always have the same format (hash + specific identifier), explain to the user we don't know that directive. List the possible directives - column, line, filepath, error name, description;
- If the usage of the directives is wrong, explain to the user why it is wrong, what is expected - column, line, filepath, error name, description;
- In general always try to provide an explanation to why the private/internal API failed, what is the problem, what is the solution.

### Expected tokens

- ENUM
- SETUP
- SCOPE
- COMMA
- PRIV
- MAPPING
- FOR
- INSERT
- ITERATION
- ASLVALUE
- MACRO
- SET
- STRING
- INTEGER
- BOOLEAN
- FLOAT
- TIMESTAMP
- INCLUDE
- NAMESPACE
- COLON
- EQUALS
- ARROW
- RARROW
- BANG
- SEMICOLON
- DOT
- LBRACE
- RBRACE
- LPAREN
- RPAREN
- LBRACKET
- RBRACKET
- PIPE
- OPTIONAL
- STRING_LITERAL
- INTEGER_LITERAL
- FLOAT_LITERAL
- BOOLEAN_LITERAL
- MULTILINE_STRING
- IDENTIFIER
- HASH
- EOF
- ASSERT
- IF
- AND
- OR
- MATCH
- LEN
- NOT
- ERROR
- EQ
- NEQ
- LE
- GE
- LT
- GT

### Multiline processing order

When implementing the `#multiline` directive, apply the arguments in the following order:

1. **`no_indent`:** Strip common leading whitespace across all lines. Find the minimum indentation among non-empty content lines and remove that many characters from each line.
2. **`no_newline`:** Replace all newlines with single spaces.
3. **`strip_spaces`:** Collapse all whitespace sequences (tabs, spaces, newlines) into single spaces, then trim leading/trailing whitespace.

Applying the arguments in this order ensures predictable results when multiple arguments are combined.

## Public API

The main function that is required is the `load_something_file`, which should accept a filepath to the `<file>.something` and returns an evaluated object/struct that can be used by all the other functions.

The public API functions should fail with relevant information for the user calling the function. For instance:

- We are trying to access a path, but somewhere along the way we don't find it, we should keep track of all the segments that are correct and report an error saying that the specific segment failed.
- If we are trying to access by a specific index, we should also report an error based on the amount of things we found. If we have 2 elements, and we try to access index 3, we should fail and say that the current configuration path only has 2 elements and we are out of bounds.
- When we try to get definition, if we don't find the definition we must say that there is no such definition and use the same approach as previously, indicating where in the path we were not able to find, or if the final location where the definition is supposed to be is not found.

### Path wildcard for `#iteration` keys

Because SOMETHING variables generated by `#iteration` have auto-named keys (`iteration_0000000000`, `iteration_0000000001`, etc.), the public API uses a special wildcard segment `[iteration]` that matches any key whose name follows the pattern:

```
iteration_<10-digit-counter><optional-label>
```

For example:

- `iteration_0000000000` matches `[iteration]`
- `iteration_0000000005_label` matches `[iteration]_label`
- `iteration_0000000000` does not match `[iteration]_label` (the label suffix must be present when specified)

Each family of functions can handle nested `#iteration` directives (with and without labels). Meaning, if we have something like:

```something
#iteration: scope = {
    #iteration("_label_1"): scope = {
        #iteration("_label_2"): scope = {
            a := "1234";
        }
    }
}
```

We can use any of the functions like this (using `get_something_val_string_once` for the sake of example):

```
// Config is variable containing the previous config.
    result := get_something_val_string_once(config, "[iteration]", "[iteration]_label_1", "[iteration]_label_2", "a"); // Should return "1234"
```

### Family `get_something_val_*_once`

These types of functions are used to get a very specific field, and only the first occurrence of that field. Since SOMETHING has the `#iteration` directive, it accounts for that while doing the search.

All of these functions behave similarly, they expect a configuration object/struct/class from the public API `load_something_file`, we need to pass it as the first argument for each function. In all cases, this `config` object will act as an immutable source of truth for the config and should not be changed at runtime, just looked at.

The config is already loaded and available to use. The path_to_setup is either a list of strings or a variadic argument of strings. Each segment defines the path towards the value we want. Depending on whether the language is dynamic, we can return whatever; otherwise, we have different ways to handle that. We can use generic arguments from the language, we can return a void pointer and cast to the correct class/struct. We can have a specific overload that returns that type of setup based. Implementation details will vary from language to language.

Also, since the internal/private API is already responsible for typechecking and making sure all the fields have values, and they are not missing, we don't need fallbacks.

- `get_something_val_setup_once(config: <config_type>, path_to_setup: list<strings>): <setup>`, just the setup value, not the variable storing that mapping;
- `get_something_val_mapping_once(config:<config_type>, path_to_mapping: list<strings>): <mapping>`, just the mapping value, not the variable storing that mapping;
- `get_something_val_array_once(config:<config_type>, path_to_array: list<strings>): <array>`, just the array value, not the variable storing that mapping;
- `get_something_val_scope_once(config:<config_type>, path_to_scope: list<strings>): <scope>`, include the name of the scope and all the things inside of the scope;
- `get_something_val_string_once(config:<config_type>, path_to_string: list<strings>): <string>`;
- `get_something_val_integer_once(config:<config_type>, path_to_integer: list<strings>): <integer>`;
- `get_something_val_float_once(config:<config_type>, path_to_float: list<strings>): <float>`;
- `get_something_val_timestamp_once(config:<config_type>, path_to_timestamp: list<strings>): <string>`, string with the required formatting;
- `get_something_val_boolean_once(config:<config_type>, path_to_boolean: list<strings>): <boolean>`.

### Family `get_something_val_*_index`

- `get_something_val_setup_index(config: <config_type>,index: <int/size_t>,  path_to_setup: list<strings>): <setup>`, just the setup value at a certain index, not the variable storing that mapping;
- `get_something_val_mapping_index(config:<config_type>, index: <int/size_t>, path_to_mapping: list<strings>): <mapping>`, just the mapping value at a certain index, not the variable storing that mapping;
- `get_something_val_array_index(config:<config_type>, index: <int/size_t>, path_to_array: list<strings>): <array>`, just the array value at a certain index, not the variable storing that mapping;
- `get_something_val_scope_index(config:<config_type>, index: <int/size_t>, path_to_scope: list<strings>): <scope>`, include the name of the scope and all the things inside of the scope at a certain index;
- `get_something_val_string_index(config:<config_type>, index: <int/size_t>, path_to_string: list<strings>): <string>`, string at a certain index;
- `get_something_val_integer_index(config:<config_type>, index: <int/size_t>, path_to_integer: list<strings>): <integer>`, integer at a certain index;
- `get_something_val_float_index(config:<config_type>, index: <int/size_t>, path_to_float: list<strings>): <float>`, float at a certain index;
- `get_something_val_timestamp_index(config:<config_type>, index: <int/size_t>, path_to_timestamp: list<strings>): <string>`, string at a certain index, with the required formatting;
- `get_something_val_boolean_index(config:<config_type>, index: <int/size_t>, path_to_boolean: list<strings>): <boolean>`, boolean at a certain index.

### Family `get_something_val_*_all`

- `get_something_val_setup_all(config: <config_type>, path_to_setup: list<strings>): list<setup>`, just the setup value, not the variable storing that mapping;
- `get_something_val_mapping_all(config:<config_type>, path_to_mapping: list<strings>): list<mapping>`, just the mapping value, not the variable storing that mapping;
- `get_something_val_array_all(config:<config_type>, path_to_array: list<strings>): list<array>`, just the array value, not the variable storing that mapping;
- `get_something_val_scope_all(config:<config_type>, path_to_scope: list<strings>): list<scope>`, include the name of the scope and all the things inside of the scope;
- `get_something_val_string_all(config:<config_type>, path_to_string: list<strings>): list<string>`;
- `get_something_val_integer_all(config:<config_type>, path_to_integer: list<strings>): list<integer>`;
- `get_something_val_float_all(config:<config_type>, path_to_float: list<strings>): list<float>`;
- `get_something_val_timestamp_all(config:<config_type>, path_to_timestamp: list<strings>): list<string>`, string with the required formatting;
- `get_something_val_boolean_all(config:<config_type>, path_to_boolean: list<strings>): list<boolean>`.

### Family `get_something_definition_*`

- `get_something_definition_setup(config: <config_type>, path_to_setup: list<string>): <setup_definition>`;
- `get_something_definition_enum(config: <config_type>, path_to_enum: list<string>): <enum_definition>`.
