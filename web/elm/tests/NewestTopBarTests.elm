module NewestTopBarTests exposing (all, userWithEmail, userWithId, userWithName, userWithUserName)

import Concourse
import Dict
import Html.Styled exposing (toUnstyled)
import NewestTopBar
import QueryString
import Routes
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, containing, id, style, tag, text, class)
import Html.Attributes as Attr
import Expect


userWithId : Concourse.User
userWithId =
    { id = "some-id", email = "", name = "", userName = "", teams = Dict.empty }


userWithEmail : Concourse.User
userWithEmail =
    { id = "some-id", email = "some-email", name = "", userName = "", teams = Dict.empty }


userWithName : Concourse.User
userWithName =
    { id = "some-id", email = "some-email", name = "some-name", userName = "", teams = Dict.empty }


userWithUserName : Concourse.User
userWithUserName =
    { id = "some-id", email = "some-email", name = "some-name", userName = "some-user-name", teams = Dict.empty }


all : Test
all =
    describe "TopBar"
        [ describe "userDisplayName"
            [ test "concourse logo is visible on top bar" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has
                            [ style
                                [ ( "background-image", "url(/public/images/concourse_logo_white.svg)" )
                                , ( "background-position", "50% 50%" )
                                , ( "background-repeat", "no-repeat" )
                                , ( "background-size", "42px 42px" )
                                , ( "width", "54px" )
                                , ( "height", "54px" )
                                ]
                            ]
            , test "top bar renders pipeline breadcrumb selector" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.has [ id "breadcrumb-pipeline" ]
            , test "top bar has pipeline breadcrumb with icon rendered first" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has pipelineBreadcrumbSelector
            , test "top bar has pipeline name after pipeline icon" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has
                            [ text "pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page when viewing job details" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Job "team" "pipeline" "job", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "there is a / between pipeline and job in breadcrumb" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.findAll [ tag "li" ]
                        |> Expect.all
                            ([ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                             , Query.index 0 >> Query.has [ id "breadcrumb-pipeline" ]
                             , Query.index 2 >> Query.has [ id "breadcrumb-job" ]
                             ]
                            )
            , test "job breadcrumb is laid out horizontally with appropriate spacing" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-job" ]
                        |> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , test "top bar has job breadcrumb with job icon rendered first" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-job" ]
                        |> Query.has jobBreadcrumbSelector
            , test "top bar has build name after job icon" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-job" ]
                        |> Query.has
                            [ text "job" ]
            , test "there is a / between pipeline and resource in breadcrumb" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.findAll [ tag "li" ]
                        |> Expect.all
                            ([ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                             , Query.index 1 >> Query.has [ text "/" ]
                             , Query.index 2 >> Query.has [ id "breadcrumb-resource" ]
                             ]
                            )
            , test "resource breadcrumb is laid out horizontally with appropriate spacing" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-resource" ]
                        |> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , test "top bar has resource breadcrumb with resource icon rendered first" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-resource" ]
                        |> Query.children []
                        |> Query.index 0
                        |> Query.has resourceBreadcrumbSelector
            , test "top bar has resource name after resource icon" <|
                \_ ->
                    NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> NewestTopBar.view
                        |> toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ id "breadcrumb-resource" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has
                            [ text "resource" ]
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
