module Views.CommentBarTests exposing (all)

import Expect
import Message.Effects exposing (Effect(..), toHtmlID)
import Message.Message exposing (CommentBarButtonKind(..), DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Test exposing (Test, describe, test)
import Views.CommentBar exposing (Model, State(..), defaultStyle, getContent, getTextareaID, handleDelivery, setCachedContent, update)


all : Test
all =
    describe "comment bar"
        [ describe "update / view methods"
            [ describe "handleDelivery"
                [ test "synchronizes text area when window is resized" <|
                    \_ ->
                        model (Viewing "")
                            |> handleDelivery (WindowResized 1.0 1.0)
                            |> (\( m, effs ) -> Expect.equal [ SyncTextareaHeight (getTextareaID m) ] effs)
                ]
            , describe "update"
                (let
                    saveComment =
                        \content -> SetBuildComment 0 content
                 in
                 [ test "clicking edit button transitions from 'Viewing' to 'Editing'" <|
                    \_ ->
                        model (Viewing "")
                            |> update (Click (CommentBarButton Edit BuildComment)) saveComment
                            |> Tuple.first
                            |> (\m -> Expect.equal (Editing { content = "", cached = "" }) m.state)
                 , test "clicking edit button focuses text area" <|
                    \_ ->
                        model (Viewing "")
                            |> update (Click (CommentBarButton Edit BuildComment)) saveComment
                            |> (\( m, effs ) -> Expect.equal [ Focus <| toHtmlID <| getTextareaID m ] effs)
                 , test "clicking save button (with changes) triggers save event" <|
                    \_ ->
                        model (Editing { content = "data", cached = "" })
                            |> update (Click (CommentBarButton Save BuildComment)) saveComment
                            |> Tuple.second
                            |> Expect.equal [ SetBuildComment 0 "data" ]
                 , test "clicking save button (with changes) sets state to 'Saving'" <|
                    \_ ->
                        model (Editing { content = "data", cached = "" })
                            |> update (Click (CommentBarButton Save BuildComment)) saveComment
                            |> Tuple.first
                            |> (\m -> Expect.equal (Saving { content = "data", cached = "" }) m.state)
                 , test "clicking save button (without changes) doesn't trigger save event" <|
                    \_ ->
                        model (Editing { content = "data", cached = "data" })
                            |> update (Click (CommentBarButton Save BuildComment)) saveComment
                            |> Tuple.second
                            |> Expect.equal []
                 , test "clicking save button (without changes) resets state to 'Viewing'" <|
                    \_ ->
                        model (Editing { content = "", cached = "" })
                            |> update (Click (CommentBarButton Save BuildComment)) saveComment
                            |> Tuple.first
                            |> (\m -> Expect.equal (Viewing "") m.state)
                 , test "editing contents updates state" <|
                    \_ ->
                        model (Editing { content = "", cached = "" })
                            |> update (EditCommentBar BuildComment "test") saveComment
                            |> Tuple.first
                            |> (\m -> Expect.equal (Editing { content = "test", cached = "" }) m.state)
                 , test "editing contents synchronizes text area" <|
                    \_ ->
                        model (Editing { content = "", cached = "" })
                            |> update (EditCommentBar BuildComment "test") saveComment
                            |> (\( m, effs ) -> Expect.equal [ SyncTextareaHeight <| getTextareaID m ] effs)
                 ]
                )
            ]
        , describe "miscellaneous methods"
            [ describe "getContent"
                [ test "State = Viewing" <|
                    \_ ->
                        model (Viewing "hello-world")
                            |> getContent
                            |> Expect.equal "hello-world"
                , test "State = Editing" <|
                    \_ ->
                        model (Editing { content = "abc-123", cached = "" })
                            |> getContent
                            |> Expect.equal "abc-123"
                , test "State = Saving" <|
                    \_ ->
                        model (Saving { content = "hello-worlds", cached = "" })
                            |> getContent
                            |> Expect.equal "hello-worlds"
                ]
            , describe "setCachedContent"
                (let
                    runTest =
                        \expected ->
                            setCachedContent expected
                                >> (\m ->
                                        case m.state of
                                            Viewing _ ->
                                                Expect.pass

                                            Editing { cached } ->
                                                Expect.equal expected cached

                                            Saving { cached } ->
                                                Expect.equal expected cached
                                   )
                 in
                 [ test "State = Viewing" <|
                    \_ ->
                        model (Viewing "hello-world")
                            |> runTest "hello-world"
                 , test "State = Editing" <|
                    \_ ->
                        model (Editing { content = "abc-123", cached = "" })
                            |> runTest "abc-123"
                 , test "State = Saving" <|
                    \_ ->
                        model (Saving { content = "hello-worlds", cached = "" })
                            |> runTest "hello-world"
                 ]
                )
            ]
        ]


model : State -> Model
model state =
    { id = BuildComment
    , state = state
    , style = defaultStyle
    }
