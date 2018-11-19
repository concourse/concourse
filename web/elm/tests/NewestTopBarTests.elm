module NewestTopBarTests exposing (all)

import Html.Styled exposing (toUnstyled)
import NewestTopBar
import QueryString
import Routes
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, containing, id, style, tag, text, class)
import Html.Attributes as Attr
import Expect exposing (..)


rspecStyleDescribe : String -> model -> List (model -> Test) -> Test
rspecStyleDescribe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


all : Test
all =
    describe "NewestTopBar"
        [ rspecStyleDescribe "rendering top bar on pipeline page"
            (NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "concourse logo is visible on top bar" <|
                Query.children []
                    >> Query.index 1
                    >> Query.has
                        [ style
                            [ ( "background-image", "url(/public/images/concourse_logo_white.svg)" )
                            , ( "background-position", "50% 50%" )
                            , ( "background-repeat", "no-repeat" )
                            , ( "background-size", "42px 42px" )
                            , ( "width", "54px" )
                            , ( "height", "54px" )
                            ]
                        ]
            , it "top bar renders pipeline breadcrumb selector" <|
                Query.has [ id "breadcrumb-pipeline" ]
            , it "top bar has pipeline breadcrumb with icon rendered first" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.first
                    >> Query.has pipelineBreadcrumbSelector
            , it "top bar has pipeline name after pipeline icon" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has
                        [ text "pipeline" ]
            , it "pipeline breadcrumb should have a link to the pipeline page" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            ]
        , rspecStyleDescribe "rendering top bar on build page"
            (NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "job breadcrumb is laid out horizontally with appropriate spacing" <|
                Query.find [ id "breadcrumb-job" ]
                    >> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , it "top bar has job breadcrumb with job icon rendered first" <|
                Query.find [ id "breadcrumb-job" ]
                    >> Query.has jobBreadcrumbSelector
            , it "top bar has build name after job icon" <|
                Query.find [ id "breadcrumb-job" ]
                    >> Query.has [ text "job" ]
            ]
        , rspecStyleDescribe "rendering top bar on resource page"
            (NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "there is a / between pipeline and resource in breadcrumb" <|
                Query.findAll [ tag "li" ]
                    >> Expect.all
                        ([ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                         , Query.index 1 >> Query.has [ text "/" ]
                         , Query.index 2 >> Query.has [ id "breadcrumb-resource" ]
                         ]
                        )
            , it "resource breadcrumb is laid out horizontally with appropriate spacing" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , it "top bar has resource breadcrumb with resource icon rendered first" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.children []
                    >> Query.index 0
                    >> Query.has resourceBreadcrumbSelector
            , it "top bar has resource name after resource icon" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has
                        [ text "resource" ]
            ]
        , rspecStyleDescribe "rendering top bar on job page"
            (NewestTopBar.init { logical = Routes.Job "team" "pipeline" "job", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing job details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "there is a / between pipeline and job in breadcrumb" <|
                Query.findAll [ tag "li" ]
                    >> Expect.all
                        ([ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                         , Query.index 0 >> Query.has [ id "breadcrumb-pipeline" ]
                         , Query.index 2 >> Query.has [ id "breadcrumb-job" ]
                         ]
                        )
            ]
        ]


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic_breadcrumb_pipeline.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic_breadcrumb_job.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic_breadcrumb_resource.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]
