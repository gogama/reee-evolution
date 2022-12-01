# reee-evolution
Email filter for Evolution

JavaScript: https://github.com/dop251/goja

Rough design of JavaScript part.

    --- init ---
    1. Create an instance of Runtime. https://pkg.go.dev/github.com/dop251/goja#Runtime
    2. Export a rule registration hook into the Runtime using Runtime.Set
    3. Load the user's rules.js files from ~/.config/reee/rules.js and compile it,
       using its containing directory as the CWD for the opration. The assumption
       is that compiling it will cause it to load other dependent files. Otherwise
       probably need to also implement some kind of require/import directive,
       whatever is normal for JS.
    4. Run the compiled program. The assumption is this should allow the JS
       program to register the groups/rules back using the rule registration
       hook, #2.

    --- rule evaluation ---
    1. Create proxy/wrapper values for Msg and Tagger. ("Marshalling")
    2. Call the designated JS callable that was registered for that rule.
    3. Handle any error and the return value which should be a bool. ("Unmarshalling")
    
    (Because of the concurrency issue below we might want to have dynamic binding
     from "JSRule" Go objects to the actual JS callables. However, that's a
     problem also because having multiple Runtimes implies the JS code might
     get reloaded several times which implies the groups and rules could change,
     which is very unpleasant. So it seems like the right thing to do is just
     serialize individual rule evaluation calls thru one Runtime tknowing it's a bottleneck.)
    

    --- issues ---
    for "true" multi-programming we probably need a bunch of Runtimes because
    they are not safe for concurrency and there's a risk that the goja library's
    bookkeeping could break if several goroutines use them at a time.