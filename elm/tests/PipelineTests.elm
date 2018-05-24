module PipelineTests exposing (..)

import Char
import Expect exposing (..)
import Keyboard
import Pipeline exposing (update, Msg(..))
import QueryString
import RemoteData exposing (WebData)
import Routes
import Test exposing (..)
import Time exposing (Time)


all : Test
all =
    describe "Pipeline"
        [ describe "update" <|
            let
                resetFocus =
                    (\_ -> Cmd.map (\_ -> Noop) Cmd.none)

                defaultModel : Pipeline.Model
                defaultModel =
                    { ports = { render = (\( _, _ ) -> Cmd.none), title = (\_ -> Cmd.none) }
                    , pipelineLocator = { teamName = "some-team", pipelineName = "some-pipeline" }
                    , pipeline = RemoteData.NotAsked
                    , fetchedJobs = Nothing
                    , fetchedResources = Nothing
                    , renderedJobs = Nothing
                    , renderedResources = Nothing
                    , concourseVersion = "some-version"
                    , turbulenceImgSrc = "some-turbulence-img-src"
                    , experiencingTurbulence = False
                    , route = { logical = Routes.Pipeline "" "", queries = QueryString.empty, page = Nothing, hash = "" }
                    , selectedGroups = []
                    , hideLegend = False
                    , hideLegendCounter = 0
                    }
            in
                [ test "HideLegendTimerTicked" <|
                    \_ ->
                        Expect.equal
                            (1 * Time.second)
                        <|
                            .hideLegendCounter <|
                                Tuple.first <|
                                    update (HideLegendTimerTicked 0) defaultModel
                , test "HideLegendTimeTicked reaches timeout" <|
                    \_ ->
                        Expect.equal
                            True
                        <|
                            .hideLegend <|
                                Tuple.first <|
                                    update (HideLegendTimerTicked 0) { defaultModel | hideLegendCounter = 10 * Time.second }
                , test "ShowLegend" <|
                    \_ ->
                        let
                            updatedModel =
                                Tuple.first <|
                                    update ShowLegend { defaultModel | hideLegend = True, hideLegendCounter = 3 * Time.second }
                        in
                            Expect.equal
                                ( False, 0 )
                            <|
                                ( .hideLegend updatedModel, .hideLegendCounter updatedModel )
                , test "KeyPressed" <|
                    \_ ->
                        Expect.equal
                            ( defaultModel, Cmd.none )
                        <|
                            update (KeyPressed (Char.toCode 'a')) defaultModel
                , test "KeyPressed f" <|
                    \_ ->
                        Expect.notEqual
                            ( defaultModel, Cmd.none )
                        <|
                            update (KeyPressed (Char.toCode 'f')) defaultModel
                ]
        ]
