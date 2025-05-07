package vars

type TemplateResolver struct {
	configPayload []byte
	params        []Variables
}

// Creates a template resolver, given a configPayload and a slice of param sources. If more than
// one param source is specified, they will be tried for variable lookup in the provided order.
// See implementation of NewMultiVars for details.
func NewTemplateResolver(configPayload []byte, params []Variables) TemplateResolver {
	return TemplateResolver{
		configPayload: configPayload,
		params:        params,
	}
}

func (resolver TemplateResolver) Resolve(expectAllKeys bool) ([]byte, error) {
	tpl := NewTemplate(resolver.configPayload)
	bytes, err := tpl.Evaluate(NewMultiVars(resolver.params), EvaluateOpts{ExpectAllKeys: expectAllKeys})
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
